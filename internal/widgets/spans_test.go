package widgets

import (
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/cachestats"
	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/gitcache"
	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/render"
	"github.com/joegoldin/agent-statusline/internal/transcript"
	"github.com/joegoldin/agent-statusline/internal/voice"
)

// everySpanWidget lists the widgets converted so far. A widget lands here the
// moment it implements SpanRenderer, and the round-trip below keeps the two
// code paths honest.
func everySpanWidget() []Widget {
	return []Widget{
		Model{}, CWD{}, Git{}, Duration{}, Tokens{}, Voice{},
		Compaction{}, PR{}, Cost{}, Effort{}, SessionName{},
		ContextBar{}, Usage5h{}, Usage7d{}, BurnRate{},
		AutoMode{}, Cache{},
	}
}

func TestSpansRoundTripToRenderOutput(t *testing.T) {
	ctx := fixtureContext(t)
	for _, w := range everySpanWidget() {
		text, visible := SafeRender(w, ctx)
		spans, spanVisible := SafeRenderSpans(w, ctx)
		if visible != spanVisible {
			t.Errorf("%s: Render visible=%v, RenderSpans visible=%v", w.Name(), visible, spanVisible)
			continue
		}
		if got := spans.ANSI(); got != text {
			t.Errorf("%s: spans.ANSI() = %q, Render() = %q", w.Name(), got, text)
		}
	}
}

func TestSpansCarryNoRawEscapes(t *testing.T) {
	ctx := fixtureContext(t)
	for _, w := range everySpanWidget() {
		spans, visible := SafeRenderSpans(w, ctx)
		if !visible {
			continue
		}
		for _, s := range spans {
			for _, r := range s.Text {
				if r == 0x1b {
					t.Errorf("%s: span text contains a raw escape: %q", w.Name(), s.Text)
					break
				}
			}
			if s.Kind == "text" && s.Intent == "" {
				t.Errorf("%s: text span %q has no intent", w.Name(), s.Text)
			}
		}
	}
}

type legacyOnlyWidget struct{}

func (legacyOnlyWidget) Name() string                   { return "legacy" }
func (legacyOnlyWidget) Render(*Context) (string, bool) { return "legacy", true }

func TestSafeRenderSpansFallsBackForNonSpanWidget(t *testing.T) {
	spans, visible := SafeRenderSpans(legacyOnlyWidget{}, fixtureContext(t))
	if !visible {
		t.Fatal("fallback widget reported invisible")
	}
	if len(spans) != 1 || spans[0].Intent != render.IntentText || spans[0].Text != "legacy" {
		t.Errorf("fallback spans = %+v, want one text-intent span holding the raw output", spans)
	}
}

// fixtureContext makes every widget in everySpanWidget() visible at once, so
// the round-trip covers real output rather than eleven hidden widgets.
func fixtureContext(t *testing.T) *Context {
	t.Helper()
	used := 53.5
	return &Context{
		Mode: input.ModePi,
		Cfg:  config.Defaults(),
		Now:  time.Unix(1748260800, 0).UTC(),
		Status: input.Status{
			CWD:         "/home/joe/Development/agent-statusline",
			SessionName: "native-pi",
			Model:       input.Model{ID: "gpt-5.6-sol", DisplayName: "Sol", Provider: "openai-codex"},
			AutoMode:    "AM● a:105 d:4 ca:89 cd:4",
			Workspace:   input.Workspace{GitWorktree: "feature"},
			Effort:      &input.Effort{Level: "xhigh"},
			Cost:        &input.Cost{TotalCostUSD: 4.20, TotalDurationMS: 4_530_000},
			PR:          &input.PR{Number: 12, URL: "https://example.test/pr/12", ReviewState: "approved"},
			ContextWindow: &input.ContextWindow{
				ContextWindowSize: 400_000,
				UsedPercentage:    &used,
				TotalInputTokens:  214_000,
			},
			RateLimits: &input.RateLimits{
				FiveHour: &input.Window{UsedPercentage: 62, ResetsAt: 1748260800 + 2*3600},
				SevenDay: &input.Window{UsedPercentage: 71, ResetsAt: 1748260800 + 3*24*3600},
			},
		},
		CacheStatsProvider: func() *cachestats.Stats {
			return &cachestats.Stats{TotalsByModel: map[string]cachestats.Totals{
				"openai-codex/gpt-5.6-sol": {
					TotalRequests:     41,
					HitRequests:       35,
					CachedInputTokens: 1_690_000,
					TotalInputTokens:  2_130_000,
				},
			}}
		},
		GitProvider:        func() *gitcache.Git { return &gitcache.Git{Branch: "main", Dirty: true, Ahead: 2, Behind: 1} },
		VoiceProvider:      func() *voice.Config { return &voice.Config{Enabled: true, Mode: "dictate"} },
		CompactionProvider: func() int { return 3 },
		TranscriptProvider: func() *transcript.Entries {
			base := time.Unix(1748260800, 0).UTC()
			return &transcript.Entries{Requests: []transcript.Request{
				{Timestamp: base.Add(-120 * time.Second), InputTokens: 40_000},
				{Timestamp: base.Add(-30 * time.Second), InputTokens: 60_000},
			}}
		},
	}
}

func TestSpansRoundTripInCompactMode(t *testing.T) {
	ctx := fixtureContext(t)
	ctx.Width = 40 // below DefaultCompactWidth, so Compact() is true
	for _, w := range everySpanWidget() {
		text, visible := SafeRender(w, ctx)
		spans, spanVisible := SafeRenderSpans(w, ctx)
		if visible != spanVisible {
			t.Errorf("%s (compact): visible mismatch %v vs %v", w.Name(), visible, spanVisible)
			continue
		}
		if got := spans.ANSI(); got != text {
			t.Errorf("%s (compact): spans.ANSI() = %q, Render() = %q", w.Name(), got, text)
		}
	}
}

func TestContextBarEmitsAFillFractionNotAPercentage(t *testing.T) {
	ctx := fixtureContext(t)
	spans, ok := SafeRenderSpans(ContextBar{}, ctx)
	if !ok {
		t.Fatal("context widget hidden")
	}
	var bars int
	for _, s := range spans {
		if s.Kind != "bar" {
			continue
		}
		bars++
		if s.Fill < 0.534 || s.Fill > 0.536 {
			t.Errorf("bar fill = %v, want ~0.535 (a fraction, not 53.5)", s.Fill)
		}
		if s.Cells != ctx.Cfg.BarWidth {
			t.Errorf("bar cells = %d, want %d", s.Cells, ctx.Cfg.BarWidth)
		}
		if s.Style != render.BarBraille {
			t.Errorf("bar style = %q, want %q", s.Style, render.BarBraille)
		}
	}
	if bars != 1 {
		t.Errorf("got %d bar spans, want exactly 1", bars)
	}
}

func TestCompactContextDropsTheBar(t *testing.T) {
	ctx := fixtureContext(t)
	ctx.Width = 40
	spans, _ := SafeRenderSpans(ContextBar{}, ctx)
	for _, s := range spans {
		if s.Kind == "bar" {
			t.Fatal("compact context still emits a bar span")
		}
	}
}
