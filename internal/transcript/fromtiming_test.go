package transcript

import (
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/toolclock"
)

func TestFromToolTimingSplitsRunningAndRecent(t *testing.T) {
	now := time.Unix(1748260800, 0)
	timing := map[string]toolclock.Entry{
		"running": {Name: "bash", Target: "go test", StartedAt: now.Add(-5 * time.Second)},
		"done": {
			Name: "read", Target: "main.go",
			StartedAt: now.Add(-30 * time.Second),
			EndedAt:   now.Add(-10 * time.Second),
		},
	}
	e := FromToolTiming(timing, now, 5*time.Minute)

	if len(e.Tools) != 1 || e.Tools[0].ID != "running" {
		t.Fatalf("Tools = %+v, want just the running one", e.Tools)
	}
	if e.Tools[0].Name != "bash" || e.Tools[0].Target != "go test" {
		t.Errorf("running tool = %+v", e.Tools[0])
	}
	if len(e.RecentTools) != 1 || e.RecentTools[0].ID != "done" {
		t.Fatalf("RecentTools = %+v, want just the completed one", e.RecentTools)
	}
	if !e.RecentTools[0].EndedAt.Equal(now.Add(-10 * time.Second)) {
		t.Errorf("EndedAt = %v", e.RecentTools[0].EndedAt)
	}
}

func TestFromToolTimingDropsEntriesOutsideWindow(t *testing.T) {
	now := time.Unix(1748260800, 0)
	timing := map[string]toolclock.Entry{
		"old": {
			Name:      "bash",
			StartedAt: now.Add(-20 * time.Minute),
			EndedAt:   now.Add(-19 * time.Minute),
		},
	}
	e := FromToolTiming(timing, now, 5*time.Minute)
	if len(e.RecentTools) != 0 {
		t.Errorf("RecentTools = %+v, want empty outside the window", e.RecentTools)
	}
}

func TestFromToolTimingCountsCompletedByName(t *testing.T) {
	now := time.Unix(1748260800, 0)
	timing := map[string]toolclock.Entry{
		"a": {Name: "bash", StartedAt: now.Add(-9 * time.Second), EndedAt: now.Add(-8 * time.Second)},
		"b": {Name: "bash", StartedAt: now.Add(-7 * time.Second), EndedAt: now.Add(-6 * time.Second)},
		"c": {Name: "read", StartedAt: now.Add(-5 * time.Second), EndedAt: now.Add(-4 * time.Second)},
	}
	e := FromToolTiming(timing, now, 5*time.Minute)
	got := map[string]int{}
	for _, tc := range e.ToolCounts {
		got[tc.Name] = tc.Count
	}
	if got["bash"] != 2 || got["read"] != 1 {
		t.Errorf("ToolCounts = %+v, want bash=2 read=1", e.ToolCounts)
	}
}

func TestFromToolTimingIsDeterministic(t *testing.T) {
	now := time.Unix(1748260800, 0)
	timing := map[string]toolclock.Entry{
		"a": {Name: "bash", StartedAt: now.Add(-9 * time.Second)},
		"b": {Name: "read", StartedAt: now.Add(-7 * time.Second)},
		"c": {Name: "edit", StartedAt: now.Add(-5 * time.Second)},
	}
	first := FromToolTiming(timing, now, 5*time.Minute)
	for i := 0; i < 20; i++ {
		again := FromToolTiming(timing, now, 5*time.Minute)
		for j := range first.Tools {
			if first.Tools[j].ID != again.Tools[j].ID {
				t.Fatalf("map iteration leaked into output order at %d", j)
			}
		}
	}
}
