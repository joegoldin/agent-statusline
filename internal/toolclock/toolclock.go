// Package toolclock persists real tool-execution timing to a per-session
// sidecar file: written by Claude Code hooks, read by the statusline.
//
// Why it exists: the JSONL transcript records only when a tool_use is
// *emitted* (all blocks of one turn share that timestamp) and when its result
// lands — never when the tool actually starts executing. The "Waiting…"
// state (queued behind another tool, or sitting on a permission prompt) is
// pure in-memory TUI state and is never written to disk. So from the
// transcript alone a queued tool and a tool running in parallel are
// indistinguishable, and elapsed time can only be measured from emission,
// which over-counts queue + permission wait.
//
// Hooks close that gap. A PermissionRequest hook fires right before a tool
// executes (per Claude Code's per-tool lifecycle: PreToolUse → PermissionRequest
// → tool executes → PostToolUse) and stamps StartedAt; PostToolUse /
// PostToolUseFailure stamp EndedAt. Both payloads carry the tool_use_id that
// matches the transcript's tool_use block id, so the statusline joins them by
// id to render a truthful hourglass (emitted but not started), an accurate
// live elapsed (now − StartedAt), and a correct final run length
// (EndedAt − StartedAt).
package toolclock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Claude Code hook_event_name values this package reacts to.
const (
	EventPermissionRequest  = "PermissionRequest"
	EventPostToolUse        = "PostToolUse"
	EventPostToolUseFailure = "PostToolUseFailure"
)

// subdir is the per-session sidecar directory under the statusline cache root.
const subdir = "tool-timing"

// retention prunes completed entries this long after they end, so a sidecar
// can't grow without bound over a long session. Comfortably longer than the
// statusline's tool-display grace, so a just-finished tool's true duration is
// still available while it lingers on screen.
const retention = 5 * time.Minute

// maxEntries is a hard backstop against pathological growth if end events are
// ever lost (a tool whose PostToolUse never fires keeps a startedAt-only
// entry that retention can't prune).
const maxEntries = 256

// Entry is one tool's real execution window, keyed by tool_use_id. A zero
// StartedAt means the tool was emitted but hasn't begun executing yet (render
// it as waiting); a zero EndedAt means it hasn't completed yet (running).
type Entry struct {
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	// Name and Target are populated only by harnesses with no transcript of
	// their own. Claude Code leaves them empty because it joins tool names
	// from the JSONL transcript by tool_use_id; pi has no such file, so its
	// extension supplies them here and the sidecar becomes the row source.
	Name   string `json:"name,omitempty"`
	Target string `json:"target,omitempty"`
}

// sidecar is the on-disk shape: tool_use_id → Entry, wrapped so the schema can
// grow without breaking older readers.
type sidecar struct {
	Tools map[string]Entry `json:"tools"`
}

// pathFor returns the sidecar path for a session. The id is sanitized so a
// hostile/odd session id can't escape the cache directory.
func pathFor(cacheDir, sessionID string) string {
	return filepath.Join(cacheDir, subdir, sanitize(sessionID)+".json")
}

func sanitize(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, id)
}

// Record folds one hook event into the sidecar for (cacheDir, sessionID),
// keyed by toolUseID, and prunes stale entries. The whole read-modify-write
// runs under an exclusive flock on a sibling .lock file so concurrent hooks
// (parallel tools fire their PermissionRequest / PostToolUse at the same
// instant) don't clobber each other's keys; the data file itself is replaced
// atomically via rename so a lock-free reader never sees a torn write.
// Unknown events and empty ids are no-ops.
func Record(cacheDir, sessionID, event, toolUseID string, now time.Time) error {
	if cacheDir == "" || sessionID == "" || toolUseID == "" {
		return nil
	}
	switch event {
	case EventPermissionRequest, EventPostToolUse, EventPostToolUseFailure:
	default:
		return nil
	}

	p := pathFor(cacheDir, sessionID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	unlock, err := lock(p)
	if err != nil {
		return err
	}
	defer unlock()

	sc := read(p)
	e := sc.Tools[toolUseID]
	switch event {
	case EventPermissionRequest:
		// Authoritative start: fires right before the tool executes, so it
		// excludes queue + permission-prompt wait. (We deliberately do NOT
		// start on PreToolUse — that can fire up-front for a still-queued
		// tool and would re-introduce the over-counting this fixes.)
		e.StartedAt = now
	case EventPostToolUse, EventPostToolUseFailure:
		e.EndedAt = now
		// Backfill a start for any tool that completed without our seeing its
		// PermissionRequest (e.g. a hook miss), so it renders as instantly
		// done rather than stuck as a perpetual hourglass.
		if e.StartedAt.IsZero() {
			e.StartedAt = now
		}
	}
	sc.Tools[toolUseID] = e
	prune(sc, now)
	return write(p, sc)
}

// Load returns the per-tool timing map for (cacheDir, sessionID) for the
// statusline to join against transcript tools by tool_use_id. A missing or
// unreadable sidecar yields an empty map (never nil), so callers can treat a
// non-empty result as "hooks are active for this session".
func Load(cacheDir, sessionID string) map[string]Entry {
	if cacheDir == "" || sessionID == "" {
		return map[string]Entry{}
	}
	return read(pathFor(cacheDir, sessionID)).Tools
}

func read(path string) *sidecar {
	sc := &sidecar{Tools: map[string]Entry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return sc
	}
	_ = json.Unmarshal(data, sc)
	if sc.Tools == nil {
		sc.Tools = map[string]Entry{}
	}
	return sc
}

func write(path string, sc *sidecar) error {
	data, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// lock takes an exclusive advisory lock on "<path>.lock" and returns an
// unlock func. The lock file is separate from the data file so the atomic
// rename in write() (which swaps the data file's inode) can't drop the lock.
func lock(path string) (func(), error) {
	lf, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		lf.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
		_ = lf.Close()
	}, nil
}

// prune drops entries that ended more than retention ago, then enforces a hard
// cap (keeping the most recently active) as a backstop against lost end events.
func prune(sc *sidecar, now time.Time) {
	for id, e := range sc.Tools {
		if !e.EndedAt.IsZero() && now.Sub(e.EndedAt) > retention {
			delete(sc.Tools, id)
		}
	}
	if len(sc.Tools) <= maxEntries {
		return
	}
	type kv struct {
		id string
		at time.Time
	}
	all := make([]kv, 0, len(sc.Tools))
	for id, e := range sc.Tools {
		at := e.StartedAt
		if e.EndedAt.After(at) {
			at = e.EndedAt
		}
		all = append(all, kv{id, at})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at.After(all[j].at) })
	kept := make(map[string]Entry, maxEntries)
	for _, x := range all[:maxEntries] {
		kept[x.id] = sc.Tools[x.id]
	}
	sc.Tools = kept
}

// RecordStart stamps the beginning of a tool execution, carrying the tool's
// name and target because the calling harness has no transcript to join
// against. Empty identifiers are a no-op, matching Record.
func RecordStart(cacheDir, sessionID, callID, name, target string, now time.Time) error {
	return mutate(cacheDir, sessionID, callID, now, func(e *Entry) {
		e.StartedAt = now
		if name != "" {
			e.Name = name
		}
		if target != "" {
			e.Target = target
		}
	})
}

// RecordEnd stamps completion. As with Record, a missing start is backfilled
// so a lost start event never strands a row as a perpetual hourglass.
func RecordEnd(cacheDir, sessionID, callID string, now time.Time) error {
	return mutate(cacheDir, sessionID, callID, now, func(e *Entry) {
		e.EndedAt = now
		if e.StartedAt.IsZero() {
			e.StartedAt = now
		}
	})
}

// mutate runs fn against one entry under the sidecar lock, then prunes and
// writes. Both Record and the Record{Start,End} pair funnel through here so
// the on-disk format cannot diverge between harnesses.
func mutate(cacheDir, sessionID, callID string, now time.Time, fn func(*Entry)) error {
	if cacheDir == "" || sessionID == "" || callID == "" {
		return nil
	}
	p := pathFor(cacheDir, sessionID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	unlock, err := lock(p)
	if err != nil {
		return err
	}
	defer unlock()

	sc := read(p)
	e := sc.Tools[callID]
	fn(&e)
	sc.Tools[callID] = e
	prune(sc, now)
	return write(p, sc)
}
