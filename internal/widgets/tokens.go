package widgets

import (
	"fmt"
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
)

// Tokens renders the current input-token count in context. No glyph —
// the trailing " tokens" word is self-explanatory. Default format is
// "compact" (e.g. "516.9k tokens", "1.2M tokens") to match the burn-rate
// widget's human-readable style; "raw" gives the unformatted integer.
type Tokens struct{}

func (Tokens) Name() string { return "tokens" }

func (Tokens) Render(ctx *Context) (string, bool) {
	spans, ok := Tokens{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Tokens) RenderSpans(ctx *Context) (render.Spans, bool) {
	cw := ctx.Status.ContextWindow
	if cw == nil {
		return nil, false
	}
	total := cw.TotalInputTokens
	if total <= 0 && cw.CurrentUsage != nil {
		total = cw.CurrentUsage.InputTokens +
			cw.CurrentUsage.CacheCreationInputTokens +
			cw.CurrentUsage.CacheReadInputTokens
	}
	if total <= 0 {
		return nil, false
	}
	var formatted string
	if strings.EqualFold(ctx.Cfg.TokenFormat, "raw") {
		formatted = fmt.Sprintf("%d", total)
	} else {
		formatted = formatTokens(total)
	}
	// Color follows context fullness so the number itself signals how close
	// to the limit you are: dim → green → yellow → orange → red.
	pct, _ := contextPercent(ctx.Status)
	suffix := " tokens"
	if ctx.Compact() {
		suffix = " tok"
	}
	return render.Spans{render.Text(render.ThresholdIntent5(pct), formatted+suffix)}, true
}
