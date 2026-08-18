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
	c := ctx.Status.Cost
	if c == nil || c.TotalCostUSD <= 0 {
		return "", false
	}
	// Claude Max subscribers don't pay for usage inside their plan limits.
	// Only surface cost when rate limits are absent (non-Max user) or when
	// either window has reached 100% (overage territory).
	if !inOverage(ctx.Status.RateLimits) {
		return "", false
	}
	return render.Red(fmt.Sprintf("%s$%.2f", costGlyph, c.TotalCostUSD)), true
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
