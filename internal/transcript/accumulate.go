package transcript

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

// accumulator is the persistent, incrementally-updated transcript state.
// It is serialized to a per-session cache file so each render only parses
// the bytes appended since the last one — completed-tool counts therefore
// accumulate across the whole session instead of resetting with a sliding
// window, and a 12MB transcript is never re-parsed wholesale on idle ticks.
type accumulator struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"` // bytes consumed so far

	// Recent request token usage for the burn-rate EMA, pruned to a window.
	Requests []Request `json:"requests"`

	// Tools that have a tool_use but no matching tool_result yet (running).
	PendingTools map[string]Tool `json:"pending_tools"`
	PendingOrder []string        `json:"pending_order"`

	// Completed tools aggregated by name (session totals) + first-seen order.
	CompletedCounts map[string]int `json:"completed_counts"`
	CompletedOrder  []string       `json:"completed_order"`

	// RecentTools holds the most recently completed tools (with EndedAt) so a
	// finished command can linger briefly on screen instead of vanishing the
	// instant its tool_result arrives. Bounded to a handful; the widget
	// applies the precise display grace.
	RecentTools []Tool `json:"recent_tools"`

	// Subagents by tool_use id + launch order. Completed ones are pruned to
	// a recent few; running ones are always kept.
	Agents     map[string]Agent `json:"agents"`
	AgentOrder []string         `json:"agent_order"`

	// FleetView TaskCreate/TaskUpdate tracking.
	TaskByID  map[string]TodoItem `json:"task_by_id"`
	TaskOrder []string            `json:"task_order"`

	// Last TodoWrite snapshot (standard Claude todo tool).
	LastTodoWrite *TodoSnapshot `json:"last_todo_write"`

	// LastTaskActivity is when a FleetView task was last created/updated. It
	// timestamps the projected FleetView todo snapshot so the widget can drop
	// an all-complete list after a grace period, same as TodoWrite.
	LastTaskActivity time.Time `json:"last_task_activity"`

	// PeakPromptSize is the largest assistant prompt size (input + cache
	// read + cache creation tokens) seen in the current context epoch. It
	// only grows within an epoch; a sharp drop signals a compaction/resume,
	// at which point tool and agent activity is reset so the counts reflect
	// "since the last compaction" rather than the whole session.
	PeakPromptSize int `json:"peak_prompt_size"`
}

// epochDropRatio: a new prompt smaller than this fraction of the epoch peak
// is treated as a compaction/resume boundary. Real compactions drop to well
// under half; normal turns only grow the prompt, so this never false-fires
// mid-epoch.
const epochDropRatio = 0.6

// resetEpoch clears tool and agent activity at a compaction/resume boundary
// so counts reflect "since the last compaction". Todos are intentionally NOT
// reset here — they have their own lifecycle (cleared when Claude empties the
// list or a grace period after they're all done), independent of compaction.
// The request window (time-based burn rate) is also kept.
func (a *accumulator) resetEpoch() {
	a.PendingTools = map[string]Tool{}
	a.PendingOrder = nil
	a.CompletedCounts = map[string]int{}
	a.CompletedOrder = nil
	a.RecentTools = nil
	a.Agents = map[string]Agent{}
	a.AgentOrder = nil
}

// observePrompt updates the epoch peak and returns true if promptSize marks a
// compaction/resume boundary (a sharp drop from the peak).
func (a *accumulator) observePrompt(promptSize int) bool {
	if promptSize <= 0 {
		return false
	}
	if a.PeakPromptSize > 0 && float64(promptSize) < float64(a.PeakPromptSize)*epochDropRatio {
		a.PeakPromptSize = promptSize
		return true
	}
	if promptSize > a.PeakPromptSize {
		a.PeakPromptSize = promptSize
	}
	return false
}

func newAccumulator(path string) *accumulator {
	return &accumulator{
		Path:            path,
		PendingTools:    map[string]Tool{},
		CompletedCounts: map[string]int{},
		Agents:          map[string]Agent{},
		TaskByID:        map[string]TodoItem{},
	}
}

// ensureMaps defends against a cache file that was written before a map
// field existed (decodes to nil).
func (a *accumulator) ensureMaps() {
	if a.PendingTools == nil {
		a.PendingTools = map[string]Tool{}
	}
	if a.CompletedCounts == nil {
		a.CompletedCounts = map[string]int{}
	}
	if a.Agents == nil {
		a.Agents = map[string]Agent{}
	}
	if a.TaskByID == nil {
		a.TaskByID = map[string]TodoItem{}
	}
}

// Load returns the transcript Entries for path, parsing only bytes appended
// since the cached offset. cacheDir holds one JSON file per transcript path.
// now and window govern request pruning for the burn-rate EMA. A missing
// transcript yields empty Entries; a truncated/rotated file resets state.
func Load(path, cacheDir string, now time.Time, window time.Duration) (*Entries, error) {
	if path == "" {
		return &Entries{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Entries{}, nil
		}
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()

	cachePath := filepath.Join(cacheDir, hashPath(path)+".json")
	acc := loadAccumulator(cachePath, path)

	// Reset if the file shrank/rotated, or the cache is for another path.
	if acc.Path != path || size < acc.Offset {
		acc = newAccumulator(path)
	}
	acc.ensureMaps()

	if size > acc.Offset {
		if _, err := f.Seek(acc.Offset, io.SeekStart); err != nil {
			return nil, err
		}
		consumed, perr := acc.consume(f)
		if perr != nil {
			return nil, perr
		}
		acc.Offset += consumed
	}

	acc.prune(now, window)
	saveAccumulator(cachePath, acc)
	return acc.toEntries(), nil
}

// consume scans complete JSONL lines from r, classifying each into the
// accumulator, and returns the number of bytes consumed (only up to the
// last newline — a trailing partial line is left for the next render).
func (a *accumulator) consume(r io.Reader) (int64, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var consumed int64
	for {
		line, err := br.ReadBytes('\n')
		if err == io.EOF {
			// Trailing bytes without a newline are an incomplete record;
			// don't advance past them so we re-read once it's flushed.
			break
		}
		if err != nil {
			return consumed, err
		}
		consumed += int64(len(line))
		a.classifyLine(bytes.TrimRight(line, "\n"))
	}
	return consumed, nil
}

func (a *accumulator) addRequest(r Request) {
	// Streaming can rewrite the same message id; keep the latest copy.
	for i := range a.Requests {
		if a.Requests[i].ID == r.ID {
			a.Requests[i] = r
			return
		}
	}
	a.Requests = append(a.Requests, r)
}

func (a *accumulator) addPendingTool(t Tool) {
	if _, ok := a.PendingTools[t.ID]; !ok {
		a.PendingOrder = append(a.PendingOrder, t.ID)
	}
	a.PendingTools[t.ID] = t
}

func (a *accumulator) completeTool(id string, ts time.Time) {
	t, ok := a.PendingTools[id]
	if !ok {
		return
	}
	if _, seen := a.CompletedCounts[t.Name]; !seen {
		a.CompletedOrder = append(a.CompletedOrder, t.Name)
	}
	a.CompletedCounts[t.Name]++
	delete(a.PendingTools, id)
	a.PendingOrder = removeString(a.PendingOrder, id)

	// Keep the finished command around briefly (newest last) so it can linger
	// on screen instead of vanishing the instant its result lands.
	t.EndedAt = ts
	a.RecentTools = append(a.RecentTools, t)
	const maxRecentTools = 8
	if len(a.RecentTools) > maxRecentTools {
		a.RecentTools = a.RecentTools[len(a.RecentTools)-maxRecentTools:]
	}
}

func (a *accumulator) addAgent(ag Agent) {
	if _, ok := a.Agents[ag.ID]; !ok {
		a.AgentOrder = append(a.AgentOrder, ag.ID)
	}
	a.Agents[ag.ID] = ag
}

func (a *accumulator) completeAgent(id string, ts time.Time) {
	if ag, ok := a.Agents[id]; ok && !ag.Background {
		ag.EndedAt = ts
		a.Agents[id] = ag
	}
}

// forceCompleteAgent marks an agent ended regardless of Background — used for
// the async task-notification, which is a backgrounded agent's real completion
// (unlike its immediate launch tool_result, which completeAgent ignores).
func (a *accumulator) forceCompleteAgent(id string, ts time.Time) {
	if ag, ok := a.Agents[id]; ok && ag.EndedAt.IsZero() {
		ag.EndedAt = ts
		a.Agents[id] = ag
	}
}

// prune trims request history to the burn-rate window and caps the number
// of completed agents retained (running agents are always kept).
func (a *accumulator) prune(now time.Time, window time.Duration) {
	// Keep requests within 2× the window — enough for the EMA tail.
	cutoff := now.Add(-2 * window)
	kept := a.Requests[:0]
	for _, r := range a.Requests {
		if !r.Timestamp.Before(cutoff) {
			kept = append(kept, r)
		}
	}
	a.Requests = kept

	// Cap completed agents to the most recent few; always keep running.
	const maxCompleted = 5
	var running, completed []string
	for _, id := range a.AgentOrder {
		ag, ok := a.Agents[id]
		if !ok {
			continue
		}
		if ag.EndedAt.IsZero() {
			running = append(running, id)
		} else {
			completed = append(completed, id)
		}
	}
	if len(completed) > maxCompleted {
		drop := completed[:len(completed)-maxCompleted]
		for _, id := range drop {
			delete(a.Agents, id)
		}
		completed = completed[len(completed)-maxCompleted:]
	}
	// Rebuild AgentOrder preserving original ordering.
	keepSet := map[string]bool{}
	for _, id := range running {
		keepSet[id] = true
	}
	for _, id := range completed {
		keepSet[id] = true
	}
	order := a.AgentOrder[:0]
	for _, id := range a.AgentOrder {
		if keepSet[id] {
			order = append(order, id)
		}
	}
	a.AgentOrder = order
}

// toEntries projects the accumulator into the widget-facing Entries.
func (a *accumulator) toEntries() *Entries {
	e := &Entries{}
	e.Requests = append(e.Requests, a.Requests...)

	for _, id := range a.PendingOrder {
		if t, ok := a.PendingTools[id]; ok {
			e.Tools = append(e.Tools, t)
		}
	}
	e.RecentTools = append(e.RecentTools, a.RecentTools...)
	for _, name := range a.CompletedOrder {
		e.ToolCounts = append(e.ToolCounts, ToolCount{Name: name, Count: a.CompletedCounts[name]})
	}
	for _, id := range a.AgentOrder {
		if ag, ok := a.Agents[id]; ok {
			e.Agents = append(e.Agents, ag)
		}
	}

	// Todos: FleetView task stream wins when present; else last TodoWrite.
	// An emptied TodoWrite (Claude cleared the list) projects no snapshot, so
	// the widget drops the line.
	if len(a.TaskOrder) > 0 {
		snap := TodoSnapshot{Timestamp: a.LastTaskActivity}
		for _, id := range a.TaskOrder {
			snap.Todos = append(snap.Todos, a.TaskByID[id])
		}
		e.Todos = append(e.Todos, snap)
	} else if a.LastTodoWrite != nil && len(a.LastTodoWrite.Todos) > 0 {
		e.Todos = append(e.Todos, *a.LastTodoWrite)
	}
	return e
}

// --- cache file I/O ---

func hashPath(p string) string {
	h := sha256.Sum256([]byte(p))
	return hex.EncodeToString(h[:16])
}

func loadAccumulator(cachePath, transcriptPath string) *accumulator {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return newAccumulator(transcriptPath)
	}
	var a accumulator
	if json.Unmarshal(data, &a) != nil {
		return newAccumulator(transcriptPath)
	}
	a.ensureMaps()
	return &a
}

func saveAccumulator(cachePath string, a *accumulator) {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(a)
	if err != nil {
		return
	}
	tmp := cachePath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, cachePath)
	}
}

func removeString(xs []string, target string) []string {
	out := xs[:0]
	for _, x := range xs {
		if x != target {
			out = append(out, x)
		}
	}
	return out
}
