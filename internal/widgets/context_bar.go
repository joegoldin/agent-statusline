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
	pct, ok := contextPercent(ctx.Status)
	if !ok {
		return "", false
	}
	color := render.ThresholdColor5(pct)
	pctStr := color(fmt.Sprintf("%d%%", int(pct+0.5)))
	if ctx.Compact() {
		return fmt.Sprintf("%s %s", color(contextGlyph), pctStr), true
	}
	width := ctx.Cfg.BarWidth
	if width <= 0 {
		width = 10
	}
	// The bar paints a smooth per-cell rainbow; the glyph and percent text
	// use the step palette so the alarm signal stays sharp.
	bar := render.GradientBar(pct, width, render.BrailleStyle)
	return fmt.Sprintf("%s%s %s", color(contextGlyph), bar, pctStr), true
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
