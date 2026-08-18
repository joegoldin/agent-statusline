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
	spans, ok := Usage5h{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Usage5h) RenderSpans(ctx *Context) (render.Spans, bool) {
	if ctx.Status.RateLimits == nil || ctx.Status.RateLimits.FiveHour == nil {
		return nil, false
	}
	return usageWindowSpans(ctx, "5h", ctx.Status.RateLimits.FiveHour, 5*time.Hour, render.BarBlock), true
}

type Usage7d struct{}

func (Usage7d) Name() string { return "usage7d" }

func (Usage7d) Render(ctx *Context) (string, bool) {
	spans, ok := Usage7d{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Usage7d) RenderSpans(ctx *Context) (render.Spans, bool) {
	if ctx.Status.RateLimits == nil || ctx.Status.RateLimits.SevenDay == nil {
		return nil, false
	}
	w := ctx.Status.RateLimits.SevenDay
	if threshold := float64(ctx.Cfg.SevenDayThreshold); threshold > 0 && w.UsedPercentage < threshold {
		return nil, false
	}
	return usageWindowSpans(ctx, "7d", w, 7*24*time.Hour, render.BarLine), true
}

// usageWindowSpans mirrors the old renderUsageWindow exactly, one fmt segment
// at a time. The single Sprintf is decomposed because each piece carries a
// different intent: the label is plain, the percentage is threshold-coloured,
// the countdown is metadata, the pace arrow is a judgement.
func usageWindowSpans(ctx *Context, label string, w *input.Window, total time.Duration, style string) render.Spans {
	intent := render.ThresholdIntent(w.UsedPercentage)
	spans := render.Spans{render.Text(render.IntentText, usageGlyph+label+" ")}
	if !ctx.Compact() {
		width := ctx.Cfg.BarWidth
		if width <= 0 {
			width = 10
		}
		spans = append(spans,
			render.Bar(w.UsedPercentage/100, width, style),
			render.Text(render.IntentText, " "))
	}
	spans = append(spans,
		render.Text(intent, fmt.Sprintf("%d%%", int(w.UsedPercentage+0.5))),
		render.Text(render.IntentText, " ("),
		render.Text(render.IntentDim, formatCountdown(ctx.Now, time.Unix(w.ResetsAt, 0))),
		render.Text(render.IntentText, ")"))
	if pace, paceIntent, ok := paceSpan(ctx.Now, time.Unix(w.ResetsAt, 0), total, w.UsedPercentage); ok {
		spans = append(spans,
			render.Text(render.IntentText, " "),
			render.Text(paceIntent, pace))
	}
	return spans
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

// paceSpan renders ⇡ (over-consuming) / ⇣ (headroom) versus elapsed
// fraction of the window, split into its text and its intent. ok is false
// below the significance threshold.
func paceSpan(now, reset time.Time, total time.Duration, usedPct float64) (string, render.Intent, bool) {
	if total <= 0 {
		return "", "", false
	}
	elapsed := total - reset.Sub(now)
	if elapsed <= 0 || elapsed > total {
		return "", "", false
	}
	elapsedPct := float64(elapsed) / float64(total) * 100
	delta := usedPct - elapsedPct
	if delta > 2 {
		return fmt.Sprintf("⇡%d%%", int(delta+0.5)), render.IntentDanger, true
	}
	if delta < -2 {
		return fmt.Sprintf("⇣%d%%", int(-delta+0.5)), render.IntentOK, true
	}
	return "", "", false
}
