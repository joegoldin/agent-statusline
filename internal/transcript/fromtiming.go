package transcript

import (
	"sort"
	"time"

	"github.com/joegoldin/agent-statusline/internal/toolclock"
)

// FromToolTiming synthesises Entries from the tool-timing sidecar, for
// harnesses that have no transcript file of their own. Claude Code parses its
// JSONL and uses the sidecar only for accurate timing; pi has no such file, so
// its extension records tool names and targets into the sidecar and this
// function becomes the row source.
//
// Only tool rows are produced. Requests, Agents, and Todos stay empty, so
// widgets that depend on them (burn rate, which needs per-request token
// counts) hide rather than render invented numbers.
func FromToolTiming(timing map[string]toolclock.Entry, now time.Time, window time.Duration) *Entries {
	e := &Entries{}
	if len(timing) == 0 {
		return e
	}

	counts := map[string]int{}
	for id, t := range timing {
		tool := Tool{
			ID:        id,
			Name:      t.Name,
			Target:    t.Target,
			Timestamp: t.StartedAt,
			EndedAt:   t.EndedAt,
		}
		if t.EndedAt.IsZero() {
			e.Tools = append(e.Tools, tool)
			continue
		}
		if window > 0 && now.Sub(t.EndedAt) > window {
			continue
		}
		e.RecentTools = append(e.RecentTools, tool)
		counts[t.Name]++
	}

	// Map iteration order is randomised, and these slices drive rendering, so
	// sort explicitly or the statusline would reshuffle rows every refresh.
	byRecency := func(s []Tool) {
		sort.Slice(s, func(i, j int) bool {
			if !s[i].Timestamp.Equal(s[j].Timestamp) {
				return s[i].Timestamp.Before(s[j].Timestamp)
			}
			return s[i].ID < s[j].ID
		})
	}
	byRecency(e.Tools)
	byRecency(e.RecentTools)

	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		e.ToolCounts = append(e.ToolCounts, ToolCount{Name: n, Count: counts[n]})
	}
	return e
}
