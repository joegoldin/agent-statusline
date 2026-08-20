package widgets

import (
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/toolclock"
	"github.com/joegoldin/agent-statusline/internal/transcript"
)

func activityCtx(now time.Time, e *transcript.Entries, timing map[string]toolclock.Entry) *Context {
	return &Context{
		Now:                now,
		TranscriptProvider: func() *transcript.Entries { return e },
		ToolTimingProvider: func() map[string]toolclock.Entry { return timing },
	}
}

func TestActivitySnapshotEmitsAbsoluteTimestampsNotElapsed(t *testing.T) {
	now := time.Unix(1748260800, 0).UTC()
	started := now.Add(-42 * time.Second)
	snap := BuildActivitySnapshot(activityCtx(now,
		&transcript.Entries{Tools: []transcript.Tool{{ID: "c1", Name: "bash", Target: "bun test", Timestamp: started}}},
		map[string]toolclock.Entry{"c1": {StartedAt: started, Name: "bash", Target: "bun test"}},
	))
	if len(snap.Tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(snap.Tools))
	}
	got := snap.Tools[0]
	if got.StartedAtMs != started.UnixMilli() {
		t.Errorf("StartedAtMs = %d, want %d", got.StartedAtMs, started.UnixMilli())
	}
	if got.EndedAtMs != 0 {
		t.Errorf("EndedAtMs = %d, want 0 while running", got.EndedAtMs)
	}
	if got.State != "running" {
		t.Errorf("State = %q, want running", got.State)
	}
	if got.Target != "bun test" {
		t.Errorf("Target = %q", got.Target)
	}
}

func TestActivitySnapshotMarksQueuedToolsWaiting(t *testing.T) {
	now := time.Unix(1748260800, 0).UTC()
	live := now.Add(-10 * time.Second)
	snap := BuildActivitySnapshot(activityCtx(now,
		&transcript.Entries{Tools: []transcript.Tool{
			{ID: "running", Name: "bash", Timestamp: live},
			{ID: "queued", Name: "read", Timestamp: now.Add(-3 * time.Second)},
		}},
		map[string]toolclock.Entry{"running": {StartedAt: live, Name: "bash"}},
	))
	states := map[string]string{}
	for _, it := range snap.Tools {
		states[it.ID] = it.State
	}
	if states["running"] != "running" || states["queued"] != "waiting" {
		t.Errorf("states = %v, want running/waiting", states)
	}
}

func TestActivitySnapshotDoesNotApplyGraceWindows(t *testing.T) {
	// Grace filtering belongs to the renderer, which re-evaluates it every
	// second against its own clock. If the Go side filtered here, a finished
	// tool would sit on screen until the next binary invocation.
	now := time.Unix(1748260800, 0).UTC()
	old := now.Add(-10 * time.Minute)
	snap := BuildActivitySnapshot(activityCtx(now,
		&transcript.Entries{RecentTools: []transcript.Tool{{ID: "c1", Name: "read", Timestamp: old, EndedAt: old}}},
		nil,
	))
	if len(snap.Tools) != 1 {
		t.Fatalf("a long-finished tool was filtered out; got %d items", len(snap.Tools))
	}
	if snap.Tools[0].State != "done" {
		t.Errorf("State = %q, want done", snap.Tools[0].State)
	}
	if snap.Graces.ToolCompleteMs != int64(toolCompleteGrace/time.Millisecond) {
		t.Errorf("ToolCompleteMs = %d, want %d", snap.Graces.ToolCompleteMs, toolCompleteGrace/time.Millisecond)
	}
	if snap.Graces.AgentRunningStaleMs != int64(agentRunningStale/time.Millisecond) {
		t.Errorf("AgentRunningStaleMs = %d", snap.Graces.AgentRunningStaleMs)
	}
}

func TestActivitySnapshotIsWidthIndependent(t *testing.T) {
	now := time.Unix(1748260800, 0).UTC()
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "a", Name: "bash", Target: "one", Timestamp: now},
		{ID: "b", Name: "bash", Target: "two", Timestamp: now},
		{ID: "c", Name: "bash", Target: "three", Timestamp: now},
		{ID: "d", Name: "bash", Target: "four", Timestamp: now},
	}}
	narrow := activityCtx(now, e, nil)
	narrow.Width = 40
	wide := activityCtx(now, e, nil)
	wide.Width = 200
	if len(BuildActivitySnapshot(narrow).Tools) != len(BuildActivitySnapshot(wide).Tools) {
		t.Error("snapshot tool count varies with Width; capping belongs to the renderer")
	}
	for _, it := range BuildActivitySnapshot(narrow).Tools {
		if it.Target == "" {
			t.Error("target was truncated away in the snapshot")
		}
	}
}

func TestActivitySnapshotSurvivesNilTranscript(t *testing.T) {
	snap := BuildActivitySnapshot(&Context{Now: time.Unix(1748260800, 0).UTC()})
	if snap.Graces.TodoCompleteMs == 0 {
		t.Error("grace constants must ship even with no activity")
	}
	if len(snap.Tools) != 0 || snap.Todos != nil {
		t.Error("empty snapshot should be empty")
	}
}
