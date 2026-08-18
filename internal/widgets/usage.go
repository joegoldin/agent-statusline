package widgets

import (
	"fmt"
	"time"

	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/render"
)

const usageGlyph = "󰃰 " // nf-md-calendar_clock — the 5h / 7d rate-limit windows

type Usage5h struct{}

func (Usage5h) Name() string { return "usage5h" }

func (Usage5h) Render(ctx *Context) (string, bool) {
	if ctx.Status.RateLimits == nil || ctx.Status.RateLimits.FiveHour == nil {
		return "", false
	}
	return renderUsageWindow(ctx, "5h", ctx.Status.RateLimits.FiveHour, 5*time.Hour, render.BlockStyle), true
}

type Usage7d struct{}

func (Usage7d) Name() string { return "usage7d" }

func (Usage7d) Render(ctx *Context) (string, bool) {
	if ctx.Status.RateLimits == nil || ctx.Status.RateLimits.SevenDay == nil {
		return "", false
	}
	w := ctx.Status.RateLimits.SevenDay
	threshold := float64(ctx.Cfg.SevenDayThreshold)
	if threshold > 0 && w.UsedPercentage < threshold {
		return "", false
	}
	return renderUsageWindow(ctx, "7d", w, 7*24*time.Hour, render.LineStyle), true
}

func renderUsageWindow(ctx *Context, label string, w *input.Window, total time.Duration, style render.BarStyle) string {
	color := render.ThresholdColor(w.UsedPercentage)
	pct := color(fmt.Sprintf("%d%%", int(w.UsedPercentage+0.5)))
	countdown := formatCountdown(ctx.Now, time.Unix(w.ResetsAt, 0))
	pace := formatPace(ctx.Now, time.Unix(w.ResetsAt, 0), total, w.UsedPercentage)
	var bar string
	if !ctx.Compact() {
		width := ctx.Cfg.BarWidth
		if width <= 0 {
			width = 10
		}
		bar = render.GradientBar(w.UsedPercentage, width, style) + " "
	}
	out := fmt.Sprintf("%s%s %s%s (%s)", usageGlyph, label, bar, pct, render.Dim(countdown))
	if pace != "" {
		out += " " + pace
	}
	return out
}

func formatCountdown(now, reset time.Time) string {
	d := reset.Sub(now)
	if d <= 0 {
		return "now"
	}
	if d >= 24*time.Hour {
		days := int(d / (24 * time.Hour))
		return fmt.Sprintf("%dd", days)
	}
	if d >= time.Hour {
		h := int(d / time.Hour)
		m := int(d/time.Minute) - h*60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", int(d/time.Minute))
}

// formatPace renders ⇡ (over-consuming) / ⇣ (headroom) versus elapsed
// fraction of the window. Returns "" when below significance threshold.
func formatPace(now, reset time.Time, total time.Duration, usedPct float64) string {
	if total <= 0 {
		return ""
	}
	elapsed := total - reset.Sub(now)
	if elapsed <= 0 || elapsed > total {
		return ""
	}
	elapsedPct := float64(elapsed) / float64(total) * 100
	delta := usedPct - elapsedPct
	if delta > 2 {
		return render.Red(fmt.Sprintf("⇡%d%%", int(delta+0.5)))
	}
	if delta < -2 {
		return render.Green(fmt.Sprintf("⇣%d%%", int(-delta+0.5)))
	}
	return ""
}
