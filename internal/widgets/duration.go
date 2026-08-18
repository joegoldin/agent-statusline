package widgets

import (
	"fmt"
	"time"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const durationGlyph = " " // nf-fa-clock-o

type Duration struct{}

func (Duration) Name() string { return "duration" }

func (Duration) Render(ctx *Context) (string, bool) {
	spans, ok := Duration{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Duration) RenderSpans(ctx *Context) (render.Spans, bool) {
	c := ctx.Status.Cost
	if c == nil || c.TotalDurationMS <= 0 {
		return nil, false
	}
	d := time.Duration(c.TotalDurationMS) * time.Millisecond
	return render.Spans{render.Text(render.IntentDim, durationGlyph+formatDurationCoarse(d))}, true
}

// formatDurationCoarse renders the session duration at minute granularity so
// it changes at most once a minute instead of ticking every second:
//
//	"<1m", "5m", "1h12m", "2d3h".
func formatDurationCoarse(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	if d < 24*time.Hour {
		h := int(d / time.Hour)
		m := int(d/time.Minute) - h*60
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d / (24 * time.Hour))
	h := int(d/time.Hour) - days*24
	return fmt.Sprintf("%dd%dh", days, h)
}

// formatDuration returns a compact human-readable duration:
//
//	"45s", "5m30s", "1h12m", "2d3h".
//
// Resumed sessions can accumulate cost.total_duration_ms across days, so
// rolling over into a "day" unit keeps the number digestible.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	if d < time.Hour {
		m := int(d / time.Minute)
		s := int(d/time.Second) - m*60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	if d < 24*time.Hour {
		h := int(d / time.Hour)
		m := int(d/time.Minute) - h*60
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d / (24 * time.Hour))
	h := int(d/time.Hour) - days*24
	return fmt.Sprintf("%dd%dh", days, h)
}
