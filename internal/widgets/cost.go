package widgets

import (
	"fmt"

	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/render"
)

const costGlyph = " " // nf-fa-dollar

type Cost struct{}

func (Cost) Name() string { return "cost" }

func (Cost) Render(ctx *Context) (string, bool) {
	spans, ok := Cost{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Cost) RenderSpans(ctx *Context) (render.Spans, bool) {
	c := ctx.Status.Cost
	if c == nil || c.TotalCostUSD <= 0 {
		return nil, false
	}
	// Claude Max subscribers don't pay for usage inside their plan limits, so
	// in Claude mode cost only surfaces in overage territory. Under pi the auth
	// is Codex / API key / OpenRouter, where every token is billed and cost is
	// the primary meter — so it always shows.
	if ctx.Mode != input.ModePi && !inOverage(ctx.Status.RateLimits) {
		return nil, false
	}
	return render.Spans{render.Text(render.IntentDanger, fmt.Sprintf("%s$%.2f", costGlyph, c.TotalCostUSD))}, true
}

// inOverage reports whether the user is *known* to be consuming overage
// usage. Returns true ONLY when rate_limits is present and at least one
// window has hit 100%. When rate_limits is nil (either non-subscriber OR
// — much more commonly — a resumed session before the first API response
// has populated the field) we deliberately return false rather than guess,
// so Max subscribers don't see a misleading cost line during their session
// startup.
func inOverage(rl *input.RateLimits) bool {
	if rl == nil {
		return false
	}
	if rl.FiveHour != nil && rl.FiveHour.UsedPercentage >= 100 {
		return true
	}
	if rl.SevenDay != nil && rl.SevenDay.UsedPercentage >= 100 {
		return true
	}
	return false
}
