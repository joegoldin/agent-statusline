package widgets

import (
	"fmt"
	"strings"

	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/render"
)

// contextGlyph — nf-fa-cube (U+F1B2). Represents the context window as a
// fixed-size "box" we fill with conversation.
const contextGlyph = " "

type ContextBar struct{}

func (ContextBar) Name() string { return "context" }

func (ContextBar) Render(ctx *Context) (string, bool) {
	spans, ok := ContextBar{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (ContextBar) RenderSpans(ctx *Context) (render.Spans, bool) {
	pct, ok := contextPercent(ctx.Status)
	if !ok {
		return nil, false
	}
	intent := render.ThresholdIntent5(pct)
	pctText := fmt.Sprintf("%d%%", int(pct+0.5))
	if ctx.Compact() {
		return render.Spans{
			render.Text(intent, contextGlyph),
			render.Text(render.IntentText, " "),
			render.Text(intent, pctText),
		}, true
	}
	width := ctx.Cfg.BarWidth
	if width <= 0 {
		width = 10
	}
	// The bar paints a smooth per-cell ramp; the glyph and percent use the
	// step palette so the alarm signal stays sharp. Only the fraction crosses
	// the wire — the pi renderer re-derives the ramp from the active theme.
	return render.Spans{
		render.Text(intent, contextGlyph),
		render.Bar(pct/100, width, render.BarBraille),
		render.Text(render.IntentText, " "),
		render.Text(intent, pctText),
	}, true
}

// contextPercent computes the effective context percentage from Status,
// with a [1m] model-id fallback when context_window_size is missing.
func contextPercent(s input.Status) (float64, bool) {
	cw := s.ContextWindow
	if cw == nil {
		return 0, false
	}
	if cw.UsedPercentage != nil {
		return *cw.UsedPercentage, true
	}
	size := cw.ContextWindowSize
	if size == 0 && strings.Contains(strings.ToLower(s.Model.ID), "[1m]") {
		size = 1_000_000
	}
	if size == 0 {
		return 0, false
	}
	used := cw.TotalInputTokens
	if cw.CurrentUsage != nil {
		used = cw.CurrentUsage.InputTokens +
			cw.CurrentUsage.CacheCreationInputTokens +
			cw.CurrentUsage.CacheReadInputTokens
	}
	if used == 0 {
		return 0, false
	}
	return float64(used) / float64(size) * 100, true
}
