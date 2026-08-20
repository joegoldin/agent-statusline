package widgets

import (
	"sort"
	"time"
)

// ActivitySnapshot is the activity stack in structured form, for renderers
// that own their own layout and their own clock.
//
// Three things are deliberately NOT done here, because all three belong to the
// renderer and doing them in Go would freeze them between invocations:
//
//  1. no elapsed durations — only absolute epoch-millisecond timestamps, so a
//     1 Hz repaint keeps the counters climbing without respawning anything;
//  2. no grace-window filtering — the constants ship in Graces and the
//     renderer applies them against its own now, so a finished tool drops off
//     on time rather than at the next invocation;
//  3. no capping or truncation — those depend on the terminal width, which
//     only the renderer knows.
type ActivitySnapshot struct {
	Graces ActivityGraces `json:"graces"`
	Tools  []ActivityItem `json:"tools"`
	Agents []AgentItem    `json:"agents"`
	Todos  *TodoItem      `json:"todos"`
}

// ActivityGraces are the linger/staleness windows, in milliseconds, that the
// renderer applies. They live in the snapshot rather than in the renderer so
// there is one definition of "how long a finished tool lingers".
type ActivityGraces struct {
	ToolCompleteMs      int64 `json:"toolCompleteMs"`
	AgentCompleteMs     int64 `json:"agentCompleteMs"`
	AgentRunningStaleMs int64 `json:"agentRunningStaleMs"`
	TodoCompleteMs      int64 `json:"todoCompleteMs"`
}

// ActivityItem is one tool call. State is running, waiting or done — the
// distinction the sidecar makes and the transcript alone cannot.
type ActivityItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Target      string `json:"target,omitempty"`
	State       string `json:"state"`
	EmittedAtMs int64  `json:"emittedAtMs,omitempty"`
	StartedAtMs int64  `json:"startedAtMs,omitempty"`
	EndedAtMs   int64  `json:"endedAtMs,omitempty"`
}

type AgentItem struct {
	Name        string `json:"name"`
	Model       string `json:"model,omitempty"`
	Description string `json:"description,omitempty"`
	StartedAtMs int64  `json:"startedAtMs"`
	EndedAtMs   int64  `json:"endedAtMs,omitempty"`
}

// TodoItem is the latest todo snapshot, flattened. It shares a name with
// transcript.TodoItem but not a package or a shape: that one is a single
// entry, this one is the whole list reduced to a progress reading.
type TodoItem struct {
	Subject     string `json:"subject,omitempty"`
	Done        int    `json:"done"`
	Total       int    `json:"total"`
	AllComplete bool   `json:"allComplete"`
	TimestampMs int64  `json:"timestampMs,omitempty"`
}

func epochMs(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func firstNonZero(vs ...int64) int64 {
	for _, v := range vs {
		if v != 0 {
			return v
		}
	}
	return 0
}

// BuildActivitySnapshot projects the transcript plus the timing sidecar into
// the structured form. It never panics: a nil transcript yields an empty
// snapshot that still carries the grace constants.
func BuildActivitySnapshot(ctx *Context) ActivitySnapshot {
	snap := ActivitySnapshot{Graces: ActivityGraces{
		ToolCompleteMs:      int64(toolCompleteGrace / time.Millisecond),
		AgentCompleteMs:     int64(agentCompleteGrace / time.Millisecond),
		AgentRunningStaleMs: int64(agentRunningStale / time.Millisecond),
		TodoCompleteMs:      int64(todoCompleteGrace / time.Millisecond),
	}}
	entries := ctx.Transcript()
	if entries == nil {
		return snap
	}
	timing := ctx.ToolTiming()

	// Live-runner scoping is identical to Tools.Render: a tool with no
	// recorded start is only waiting if something else is genuinely running,
	// so a missed hook never strands a tool as a perpetual hourglass.
	liveRunner := false
	for _, t := range entries.Tools {
		if e, ok := timing[t.ID]; ok && !e.StartedAt.IsZero() && e.EndedAt.IsZero() {
			liveRunner = true
			break
		}
	}
	for _, t := range entries.Tools {
		e := timing[t.ID]
		state := "running"
		if e.StartedAt.IsZero() && liveRunner {
			state = "waiting"
		}
		snap.Tools = append(snap.Tools, ActivityItem{
			ID: t.ID, Name: t.Name, Target: t.Target, State: state,
			EmittedAtMs: epochMs(t.Timestamp), StartedAtMs: epochMs(e.StartedAt),
		})
	}
	for _, t := range entries.RecentTools {
		e := timing[t.ID]
		snap.Tools = append(snap.Tools, ActivityItem{
			ID: t.ID, Name: t.Name, Target: t.Target, State: "done",
			EmittedAtMs: epochMs(t.Timestamp), StartedAtMs: epochMs(e.StartedAt),
			EndedAtMs: firstNonZero(epochMs(e.EndedAt), epochMs(t.EndedAt)),
		})
	}

	for _, a := range entries.Agents {
		snap.Agents = append(snap.Agents, AgentItem{
			Name: a.Name, Model: a.Model, Description: a.Description,
			StartedAtMs: epochMs(a.StartedAt), EndedAtMs: epochMs(a.EndedAt),
		})
	}
	sort.SliceStable(snap.Agents, func(i, j int) bool {
		return snap.Agents[i].StartedAtMs > snap.Agents[j].StartedAtMs
	})

	if len(entries.Todos) > 0 {
		latest := entries.Todos[len(entries.Todos)-1]
		if len(latest.Todos) > 0 {
			item := TodoItem{Total: len(latest.Todos), TimestampMs: epochMs(latest.Timestamp)}
			for _, td := range latest.Todos {
				if td.Status == "completed" {
					item.Done++
				}
				if td.Status == "in_progress" && item.Subject == "" {
					item.Subject = td.Subject
				}
			}
			item.AllComplete = item.Done == item.Total
			snap.Todos = &item
		}
	}
	return snap
}
