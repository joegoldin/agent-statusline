package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/render"
	"github.com/joegoldin/agent-statusline/internal/toolclock"
	"github.com/joegoldin/agent-statusline/internal/transcript"
)

func actCtx(e *transcript.Entries) *Context {
	return &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                time.Unix(1_000_000, 0),
	}
}

func TestToolsShowsOnlyRunning(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "3", Name: "Edit", Target: "home.nix", Timestamp: now},
		{ID: "4", Name: "Bash", Target: "go test", Timestamp: now},
	}}
	out, vis := (&Tools{}).Render(actCtx(e))
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "Edit") || !strings.Contains(out, "home.nix") {
		t.Errorf("expected running Edit in %q", out)
	}
	if !strings.Contains(out, "Bash") || !strings.Contains(out, "go test") {
		t.Errorf("expected running Bash in %q", out)
	}
}

func TestToolsRunningShowsSpinnerAndElapsed(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "1", Name: "Bash", Target: "sleep 30", Timestamp: start},
	}}
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                start.Add(7 * time.Second),
		Width:              80,
	}
	out, vis := (&Tools{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	plain := render.StripANSI(out)
	if strings.Contains(plain, "◐") {
		t.Errorf("static half-circle should be replaced with a spinner frame: %q", plain)
	}
	frameSeen := false
	for _, f := range runningSpinnerFrames {
		if strings.Contains(plain, f) {
			frameSeen = true
			break
		}
	}
	if !frameSeen {
		t.Errorf("expected a spinner frame in %q", plain)
	}
	if !strings.Contains(plain, "(7s) Bash") {
		t.Errorf("expected '(7s)' between spinner and label in %q", plain)
	}
}

func TestToolsRunningOmitsElapsedUnderOneSecond(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "1", Name: "Read", Target: "main.go", Timestamp: start},
	}}
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                start.Add(500 * time.Millisecond),
		Width:              80,
	}
	out, _ := (&Tools{}).Render(ctx)
	plain := render.StripANSI(out)
	if strings.Contains(plain, "(0s)") {
		t.Errorf("sub-second elapsed shouldn't render a noisy (0s): %q", plain)
	}
}

func TestRunningGlyphAdvancesOncePerSecond(t *testing.T) {
	// Indexed by whole seconds: each 1s tick must advance by one frame, and
	// the full cycle must traverse every frame before wrapping.
	base := time.Unix(1_780_000_000, 0)
	prev := runningGlyph(base)
	seen := map[string]bool{prev: true}
	for s := 1; s <= len(runningSpinnerFrames); s++ {
		cur := runningGlyph(base.Add(time.Duration(s) * time.Second))
		if s < len(runningSpinnerFrames) {
			if cur == prev {
				t.Errorf("spinner stalled across a 1s tick at +%ds: %q == %q", s, prev, cur)
			}
			if seen[cur] {
				t.Errorf("spinner repeated frame %q before the cycle completed at +%ds", cur, s)
			}
			seen[cur] = true
		} else if cur != runningGlyph(base) {
			t.Errorf("spinner did not wrap after %d seconds: %q != %q", len(runningSpinnerFrames), cur, runningGlyph(base))
		}
		prev = cur
	}
}

func TestRunningGlyphHoldsWithinASecond(t *testing.T) {
	// Sub-second refreshes legitimately re-render the same frame — the spinner
	// rotates at 1 Hz regardless of how often the harness invokes the binary.
	base := time.Unix(1_780_000_000, 0)
	for _, d := range []time.Duration{0, 100 * time.Millisecond, 500 * time.Millisecond, 999 * time.Millisecond} {
		if got := runningGlyph(base.Add(d)); got != runningGlyph(base) {
			t.Errorf("spinner changed within one second (+%s): %q != %q", d, got, runningGlyph(base))
		}
	}
}

func TestRunningGlyphConstantWidthAcrossFrames(t *testing.T) {
	// The spinner pulses between a two- and three-arrow play button. Those frames
	// have different raw widths, so unless they render at the same cell width the
	// per-tool truncation budget (and the whole row's horizontal position) shifts
	// by a cell every second. Every frame must occupy the same number of cells.
	base := time.Unix(1_780_000_000, 0)
	want := render.VisibleWidth(runningGlyph(base))
	for s := 1; s <= len(runningSpinnerFrames); s++ {
		got := render.VisibleWidth(runningGlyph(base.Add(time.Duration(s) * time.Second)))
		if got != want {
			t.Errorf("spinner frame width changed at +%ds: %d != %d", s, got, want)
		}
	}
}

func TestToolsTruncationStableAcrossSpinnerFrames(t *testing.T) {
	// Regression: animating the spinner must not change how the command is
	// truncated. Two consecutive seconds land on different spinner frames but an
	// identical-width elapsed counter ("(50s)" vs "(51s)"), so the only thing
	// that varies is the play button — the middle-truncated command's trailing
	// tail must stay byte-for-byte identical between them.
	start := time.Unix(1_000_000, 0) // +50s and +51s straddle a frame flip
	long := `cd /home/joe/dotfiles && nix fmt hosts/common.nix 2>&1 | tee /tmp/OUT" | tail -30; exit 1; } && echo "BUILT: $OUT"`
	row := func(elapsed time.Duration) string {
		e := &transcript.Entries{Tools: []transcript.Tool{
			{ID: "1", Name: "Bash", Target: long, Timestamp: start},
		}}
		ctx := &Context{
			TranscriptProvider: func() *transcript.Entries { return e },
			Now:                start.Add(elapsed),
			Width:              80,
		}
		out, vis := (&Tools{}).Render(ctx)
		if !vis {
			t.Fatal("expected visible")
		}
		return render.StripANSI(out)
	}
	// Precondition: the two seconds really do select different spinner frames.
	if runningGlyph(start.Add(50*time.Second)) == runningGlyph(start.Add(51*time.Second)) {
		t.Fatal("test precondition: +50s and +51s should pick different frames")
	}
	a := row(50 * time.Second)
	b := row(51 * time.Second)
	ia := strings.Index(a, "…")
	ib := strings.Index(b, "…")
	if ia < 0 || ib < 0 {
		t.Fatalf("expected middle truncation in both:\n %q\n %q", a, b)
	}
	if a[ia:] != b[ib:] {
		t.Errorf("truncation tail shifted with the spinner frame:\n +50s: %q\n +51s: %q", a[ia:], b[ib:])
	}
}

func TestToolsSingleRunningUsesFullWidth(t *testing.T) {
	long := "cd /Users/joe/Development/dotfiles/agent-skills && nix flake update claude-nix && git commit -am bump"
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "1", Name: "Bash", Target: long, Timestamp: time.Unix(1_000_000, 0)},
	}}
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                time.Unix(1_000_000, 0),
		Width:              80,
	}
	out, vis := (&Tools{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if w := render.VisibleWidth(out); w > 80 {
		t.Errorf("row width %d exceeds 80: %q", w, out)
	} else if w < 70 {
		t.Errorf("a single tool should fill most of the line, got %d: %q", w, out)
	}
	plain := render.StripANSI(out)
	if !strings.Contains(plain, "Bash: cd /Users") {
		t.Errorf("expected command start in %q", plain)
	}
	if !strings.HasSuffix(plain, "bump") {
		t.Errorf("expected command end (middle truncation) in %q", plain)
	}
	if !strings.Contains(plain, "…") {
		t.Errorf("expected middle ellipsis in %q", plain)
	}
}

func TestToolsMultipleSplitWidthEvenly(t *testing.T) {
	long := strings.Repeat("x", 200)
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "1", Name: "Bash", Target: long, Timestamp: time.Unix(1_000_000, 0)},
		{ID: "2", Name: "Grep", Target: long, Timestamp: time.Unix(1_000_000, 0)},
	}}
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                time.Unix(1_000_000, 0),
		Width:              80,
	}
	out, _ := (&Tools{}).Render(ctx)
	if w := render.VisibleWidth(out); w > 80 {
		t.Errorf("row width %d exceeds 80: %q", w, out)
	}
	plain := render.StripANSI(out)
	if !strings.Contains(plain, "Bash") || !strings.Contains(plain, "Grep") {
		t.Errorf("expected both tools present in %q", plain)
	}
	// Both targets exceed their share → each is middle-truncated.
	if n := strings.Count(plain, "…"); n != 2 {
		t.Errorf("expected 2 truncated segments (even split), got %d: %q", n, plain)
	}
}

func TestToolsShowsRecentlyCompletedThenDrops(t *testing.T) {
	done := time.Unix(1_000_000, 0)
	e := &transcript.Entries{RecentTools: []transcript.Tool{
		{ID: "t1", Name: "Bash", Target: "go test", Timestamp: done.Add(-5 * time.Second), EndedAt: done},
	}}
	// Within the grace window → the finished command lingers, marked done.
	within := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                done.Add(3 * time.Second),
		Width:              80,
	}
	out, vis := (&Tools{}).Render(within)
	if !vis {
		t.Fatal("expected the just-completed command to stay visible within grace")
	}
	plain := render.StripANSI(out)
	if !strings.Contains(plain, "Bash") || !strings.Contains(plain, "go test") {
		t.Errorf("expected the completed command shown, got %q", plain)
	}
	if !strings.Contains(plain, doneGlyph) {
		t.Errorf("a completed command should use the done glyph, got %q", plain)
	}
	// Past the grace window → it drops.
	after := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                done.Add(toolCompleteGrace + time.Second),
		Width:              80,
	}
	if _, vis := (&Tools{}).Render(after); vis {
		t.Errorf("a completed command should drop after the grace window")
	}
}

func TestToolsCompletedShowsFinalDuration(t *testing.T) {
	done := time.Unix(1_000_000, 0)
	e := &transcript.Entries{RecentTools: []transcript.Tool{
		{ID: "t1", Name: "Bash", Target: "go test",
			Timestamp: done.Add(-12 * time.Second), EndedAt: done},
	}}
	// Render a few seconds after completion so the line is still in grace.
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                done.Add(3 * time.Second),
		Width:              80,
	}
	out, vis := (&Tools{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible within grace")
	}
	plain := render.StripANSI(out)
	// Final duration should be the actual run length (EndedAt - Timestamp),
	// not "now - start" which would keep ticking after completion.
	if !strings.Contains(plain, "(12s) Bash") {
		t.Errorf("expected '(12s)' final duration on completed tool in %q", plain)
	}
}

func TestToolsCompletedCommandFitsWidth(t *testing.T) {
	done := time.Unix(1_000_000, 0)
	long := strings.Repeat("x", 300)
	e := &transcript.Entries{RecentTools: []transcript.Tool{
		{ID: "t1", Name: "Bash", Target: long, Timestamp: done.Add(-2 * time.Second), EndedAt: done},
	}}
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                done.Add(time.Second),
		Width:              80,
	}
	out, vis := (&Tools{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	// The done glyph (✓) is two cells wide; if the budget doesn't account for
	// that, the row overflows by one and the outer end-truncate adds a stray ….
	if w := render.VisibleWidth(out); w > 80 {
		t.Errorf("completed-command row width %d exceeds 80: %q", w, out)
	}
}

func TestToolsHidesWhenNothingRunning(t *testing.T) {
	// Only completed tools (ToolCounts) → the running row hides.
	e := &transcript.Entries{ToolCounts: []transcript.ToolCount{{Name: "Read", Count: 1}}}
	if _, vis := (&Tools{}).Render(actCtx(e)); vis {
		t.Errorf("expected hidden when nothing running")
	}
	if _, vis := (&Tools{}).Render(&Context{}); vis {
		t.Errorf("expected hidden without provider")
	}
}

// timingCtx builds a context whose Tools row sees the given per-tool timing
// sidecar, at the given now and width.
func timingCtx(e *transcript.Entries, timing map[string]toolclock.Entry, now time.Time, width int) *Context {
	return &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		ToolTimingProvider: func() map[string]toolclock.Entry { return timing },
		Now:                now,
		Width:              width,
	}
}

func TestToolsWaitingShowsHourglassWithWaitTimer(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	// A is running (real start recorded); B was emitted in the same turn but
	// has no start yet → genuinely queued behind the live runner A.
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "A", Name: "Bash", Target: "running", Timestamp: now.Add(-40 * time.Second)},
		{ID: "B", Name: "Bash", Target: "queued", Timestamp: now.Add(-40 * time.Second)},
	}}
	timing := map[string]toolclock.Entry{
		"A": {StartedAt: now.Add(-20 * time.Second)},
	}
	out, vis := (&Tools{}).Render(timingCtx(e, timing, now, 120))
	if !vis {
		t.Fatal("expected visible")
	}
	plain := render.StripANSI(out)
	if !strings.Contains(plain, waitingGlyph) {
		t.Errorf("queued tool should show the hourglass in %q", plain)
	}
	// Running A: run timer from its real start (20s), not emission (40s).
	if !strings.Contains(plain, "(20s)") {
		t.Errorf("running tool should show real-start run timer (20s) in %q", plain)
	}
	// Waiting B: wait timer from emission (40s) — how long it's been queued.
	if !strings.Contains(plain, "(40s)") {
		t.Errorf("waiting tool should show wait timer from emission (40s) in %q", plain)
	}
}

func TestToolsNoDanglingHourglassWithoutLiveRunner(t *testing.T) {
	// Hooks are "active" (the sidecar has a completed entry) but nothing is
	// currently running. A pending tool with no recorded start must NOT hang
	// as a perpetual hourglass — with no live runner to queue behind, it falls
	// back to running so a missed hook can't strand it.
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "B", Name: "Bash", Target: "unhooked", Timestamp: now.Add(-5 * time.Second)},
	}}
	timing := map[string]toolclock.Entry{
		// A finished tool: started+ended, so it is NOT a live runner.
		"done": {StartedAt: now.Add(-60 * time.Second), EndedAt: now.Add(-50 * time.Second)},
	}
	out, vis := (&Tools{}).Render(timingCtx(e, timing, now, 120))
	if !vis {
		t.Fatal("expected visible")
	}
	plain := render.StripANSI(out)
	if strings.Contains(plain, waitingGlyph) {
		t.Errorf("no live runner → no hourglass; got %q", plain)
	}
	// Falls back to emission-based elapsed (5s) so the counter still works.
	if !strings.Contains(plain, "(5s)") {
		t.Errorf("expected fallback elapsed (5s) for unhooked tool in %q", plain)
	}
}

func TestToolsCancelledPhantomDoesNotStrandWaiter(t *testing.T) {
	// A cancelled tool can leave a StartedAt-only sidecar entry that never ends
	// (no PostToolUse fires on interrupt). Once it's no longer pending in the
	// transcript it must NOT count as a live runner — otherwise a newly-emitted
	// tool would be stranded as a perpetual hourglass.
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "B", Name: "Bash", Target: "new", Timestamp: now.Add(-5 * time.Second)},
	}}
	timing := map[string]toolclock.Entry{
		// "X" started long ago, never ended (cancelled), NOT in entries.Tools.
		"X": {StartedAt: now.Add(-300 * time.Second)},
	}
	out, vis := (&Tools{}).Render(timingCtx(e, timing, now, 120))
	if !vis {
		t.Fatal("expected visible")
	}
	plain := render.StripANSI(out)
	if strings.Contains(plain, waitingGlyph) {
		t.Errorf("a cancelled phantom must not strand B as waiting: %q", plain)
	}
	if !strings.Contains(plain, "(5s)") {
		t.Errorf("expected B running with fallback elapsed (5s) in %q", plain)
	}
}

func TestToolsNoHooksFallsBackToRunning(t *testing.T) {
	// With no sidecar at all (hooks not installed), pending tools render as
	// running with emission-based elapsed — never as a stuck hourglass.
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "A", Name: "Bash", Target: "x", Timestamp: now.Add(-8 * time.Second)},
		{ID: "B", Name: "Bash", Target: "y", Timestamp: now.Add(-8 * time.Second)},
	}}
	out, vis := (&Tools{}).Render(timingCtx(e, nil, now, 120))
	if !vis {
		t.Fatal("expected visible")
	}
	plain := render.StripANSI(out)
	if strings.Contains(plain, waitingGlyph) {
		t.Errorf("without hooks nothing should be waiting: %q", plain)
	}
}

func TestToolsCompletedUsesRealRunLength(t *testing.T) {
	// A finished tool prefers the hook's real start→end window over the
	// emission→result span (which would include queue + permission wait).
	now := time.Unix(1_000_000, 0)
	ended := now.Add(-3 * time.Second)
	e := &transcript.Entries{RecentTools: []transcript.Tool{
		{ID: "A", Name: "Bash", Target: "go test",
			Timestamp: now.Add(-90 * time.Second), EndedAt: ended},
	}}
	timing := map[string]toolclock.Entry{
		"A": {StartedAt: now.Add(-15 * time.Second), EndedAt: ended},
	}
	out, _ := (&Tools{}).Render(timingCtx(e, timing, now, 120))
	plain := render.StripANSI(out)
	if !strings.Contains(plain, "(12s)") {
		t.Errorf("expected real run length (12s) on completed tool in %q", plain)
	}
}

func TestToolsRecentAggregatesByName(t *testing.T) {
	e := &transcript.Entries{ToolCounts: []transcript.ToolCount{
		{Name: "Read", Count: 3},
		{Name: "Grep", Count: 1},
	}}
	out, vis := (&ToolsRecent{}).Render(actCtx(e))
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "Read") || !strings.Contains(out, "×3") {
		t.Errorf("expected Read ×3 in %q", out)
	}
	if !strings.Contains(out, "Grep") || !strings.Contains(out, "×1") {
		t.Errorf("expected Grep ×1 in %q", out)
	}
}

func TestToolsRecentFoldsMCPCalls(t *testing.T) {
	e := &transcript.Entries{ToolCounts: []transcript.ToolCount{
		{Name: "mcp__server_a__foo", Count: 3},
		{Name: "mcp__server_b__bar", Count: 2},
		{Name: "Skill", Count: 2},
		{Name: "Bash", Count: 39},
	}}
	out, vis := (&ToolsRecent{}).Render(actCtx(e))
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "MCP ×5") {
		t.Errorf("expected all mcp__* folded into MCP ×5 in %q", out)
	}
	if !strings.Contains(out, "Skill ×2") {
		t.Errorf("expected Skill ×2 alongside MCP in %q", out)
	}
	if strings.Contains(out, "mcp__") {
		t.Errorf("raw mcp__ tool names should not appear: %q", out)
	}
}

func TestToolsRecentShowsUpToFive(t *testing.T) {
	e := &transcript.Entries{ToolCounts: []transcript.ToolCount{
		{Name: "Bash", Count: 39}, {Name: "Edit", Count: 18}, {Name: "Read", Count: 11},
		{Name: "Skill", Count: 5}, {Name: "Write", Count: 3}, {Name: "Grep", Count: 2},
	}}
	out, _ := (&ToolsRecent{}).Render(actCtx(e))
	if got := strings.Count(out, "×"); got != 5 {
		t.Errorf("expected 5 columns, got %d in %q", got, out)
	}
	if strings.Contains(out, "Grep") {
		t.Errorf("lowest-count tool should be dropped past 5 columns: %q", out)
	}
}

func TestToolsRecentHidesWhenNothingCompleted(t *testing.T) {
	e := &transcript.Entries{Tools: []transcript.Tool{
		{ID: "1", Name: "Bash", Timestamp: time.Unix(1_000_000, 0)},
	}}
	if _, vis := (&ToolsRecent{}).Render(actCtx(e)); vis {
		t.Errorf("expected hidden when only running tools")
	}
}

func TestAgentsShowsRunningAndRecent(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Agents: []transcript.Agent{
		{Name: "Explore", Model: "haiku", Description: "Finding LSP",
			StartedAt: now.Add(-135 * time.Second)},
		{Name: "general-purpose", Description: "Done",
			StartedAt: now.Add(-300 * time.Second), EndedAt: now.Add(-10 * time.Second)},
	}}
	out, vis := (&Agents{}).Render(actCtx(e))
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "Explore") {
		t.Errorf("expected running Explore in %q", out)
	}
	if !strings.Contains(out, "haiku") {
		t.Errorf("expected model haiku in %q", out)
	}
	if !strings.Contains(out, "general-purpose") {
		t.Errorf("expected completed general-purpose in %q", out)
	}
}

func TestAgentsRunningShowsDimElapsedAfterSpinner(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Agents: []transcript.Agent{
		{Name: "Explore", Description: "audit",
			StartedAt: start},
	}}
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                start.Add(7 * time.Second),
		Width:              120,
	}
	out, vis := (&Agents{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	plain := render.StripANSI(out)
	// Elapsed must appear right after the spinner glyph, same shape as Tools:
	// "▶▶ (7s) Explore: …" — and never as a trailing "(7s)" after the desc.
	if !strings.Contains(plain, "(7s) Explore") {
		t.Errorf("expected '(7s) Explore' next to the spinner in %q", plain)
	}
	if strings.HasSuffix(strings.TrimSpace(plain), "(7s)") {
		t.Errorf("trailing '(7s)' should be gone from the end: %q", plain)
	}
	// The elapsed text must be dim (independent of the spinner color) so it
	// reads as metadata, matching the Tools row.
	if !strings.Contains(out, render.Dim("(7s) ")) {
		t.Errorf("expected dim '(7s) ' in %q", out)
	}
}

func TestAgentsCompletedShowsDimFinalDurationAfterCheck(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Agents: []transcript.Agent{
		{Name: "Explore", Description: "audit",
			StartedAt: start,
			EndedAt:   start.Add(12 * time.Second)},
	}}
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                start.Add(15 * time.Second),
		Width:              120,
	}
	out, vis := (&Agents{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	plain := render.StripANSI(out)
	if !strings.Contains(plain, "(12s) Explore") {
		t.Errorf("expected '(12s) Explore' (final run length) next to the done glyph in %q", plain)
	}
	if strings.HasSuffix(strings.TrimSpace(plain), "(12s)") {
		t.Errorf("trailing '(12s)' should be gone from the end: %q", plain)
	}
	if !strings.Contains(out, render.Dim("(12s) ")) {
		t.Errorf("expected dim '(12s) ' in %q", out)
	}
}

func TestAgentsHidesWhenEmpty(t *testing.T) {
	if _, vis := (&Agents{}).Render(actCtx(&transcript.Entries{})); vis {
		t.Errorf("expected hidden")
	}
}

func TestAgentsDropsCompletedPastGrace(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Agents: []transcript.Agent{
		{Name: "Explore", Description: "old work",
			StartedAt: now.Add(-10 * time.Minute),
			EndedAt:   now.Add(-agentCompleteGrace - time.Second)},
	}}
	out, vis := (&Agents{}).Render(actCtx(e))
	if vis {
		t.Errorf("a long-completed agent should drop after the grace window, got %q", out)
	}
}

func TestAgentsKeepsRecentlyCompletedWithinGrace(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Agents: []transcript.Agent{
		{Name: "Explore", Description: "just finished",
			StartedAt: now.Add(-2 * time.Minute),
			EndedAt:   now.Add(-5 * time.Second)},
	}}
	out, vis := (&Agents{}).Render(actCtx(e))
	if !vis {
		t.Fatal("expected a just-completed agent to stay visible within grace")
	}
	if !strings.Contains(out, "Explore") {
		t.Errorf("expected just-completed Explore in %q", out)
	}
}

func TestAgentsDropsCompletedButKeepsRunning(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Agents: []transcript.Agent{
		{Name: "Explore", Description: "still running",
			StartedAt: now.Add(-3 * time.Minute)},
		{Name: "general-purpose", Description: "long done",
			StartedAt: now.Add(-10 * time.Minute),
			EndedAt:   now.Add(-agentCompleteGrace - time.Second)},
	}}
	out, vis := (&Agents{}).Render(actCtx(e))
	if !vis {
		t.Fatal("expected visible (running agent present)")
	}
	if !strings.Contains(out, "Explore") {
		t.Errorf("expected running Explore in %q", out)
	}
	if strings.Contains(out, "general-purpose") {
		t.Errorf("long-completed agent should drop, got %q", out)
	}
}

func TestAgentsDropsStaleRunning(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	// An interrupted agent never stamps EndedAt, so without a cap it would spin
	// forever. Past agentRunningStale the row must drop it rather than show a
	// runaway "(17h)" elapsed.
	e := &transcript.Entries{Agents: []transcript.Agent{
		{Name: "Explore", Description: "Audit security-property coverage",
			StartedAt: now.Add(-agentRunningStale - time.Minute)},
	}}
	if out, vis := (&Agents{}).Render(actCtx(e)); vis {
		t.Errorf("a long-stuck running agent should drop past the stale cap, got %q", out)
	}
}

func TestAgentsKeepsRunningWithinStaleCap(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Agents: []transcript.Agent{
		{Name: "Explore", Description: "real work",
			StartedAt: now.Add(-agentRunningStale + time.Minute)},
	}}
	out, vis := (&Agents{}).Render(actCtx(e))
	if !vis {
		t.Fatal("a running agent under the stale cap should stay visible")
	}
	if !strings.Contains(out, "Explore") {
		t.Errorf("expected running Explore in %q", out)
	}
}

func TestTodosShowsCurrentInProgress(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Todos: []transcript.TodoSnapshot{
		{Timestamp: now, Todos: []transcript.TodoItem{
			{Subject: "wire up", Status: "completed"},
			{Subject: "write tests", Status: "in_progress"},
			{Subject: "ship it", Status: "pending"},
		}},
	}}
	out, vis := (&Todos{}).Render(actCtx(e))
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "write tests") {
		t.Errorf("expected current todo in %q", out)
	}
	if !strings.Contains(out, "1/3") {
		t.Errorf("expected 1/3 in %q", out)
	}
}

func TestTodosShowsCelebrationWhenAllDone(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Todos: []transcript.TodoSnapshot{
		{Timestamp: now, Todos: []transcript.TodoItem{
			{Subject: "a", Status: "completed"},
			{Subject: "b", Status: "completed"},
		}},
	}}
	out, vis := (&Todos{}).Render(actCtx(e))
	if !vis {
		t.Fatal("expected visible — all-done is its own affirmation")
	}
	if !strings.Contains(out, "all todos complete") {
		t.Errorf("expected 'all todos complete' message in %q", out)
	}
	if !strings.Contains(out, "2/2") {
		t.Errorf("expected 2/2 in %q", out)
	}
}

func TestTodosDropsCelebrationAfterGrace(t *testing.T) {
	done := time.Unix(1_000_000, 0)
	e := &transcript.Entries{Todos: []transcript.TodoSnapshot{
		{Timestamp: done, Todos: []transcript.TodoItem{
			{Subject: "a", Status: "completed"},
			{Subject: "b", Status: "completed"},
		}},
	}}
	ctx := &Context{
		TranscriptProvider: func() *transcript.Entries { return e },
		Now:                done.Add(todoCompleteGrace + time.Second),
	}
	if _, vis := (&Todos{}).Render(ctx); vis {
		t.Errorf("all-complete todos should drop after the grace period")
	}
}

func TestTodosHidesWhenNoTodos(t *testing.T) {
	if _, vis := (&Todos{}).Render(actCtx(&transcript.Entries{})); vis {
		t.Errorf("expected hidden when no todos")
	}
}
