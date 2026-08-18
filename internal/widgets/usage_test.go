package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/input"
)

func usageCtx(now time.Time, rl *input.RateLimits) *Context {
	return &Context{
		Status: input.Status{RateLimits: rl},
		Cfg:    config.Config{BarWidth: 4, SevenDayThreshold: 50},
		Now:    now,
	}
}

func TestUsage5hRenders(t *testing.T) {
	w := &Usage5h{}
	now := time.Unix(1_000_000, 0)
	rl := &input.RateLimits{FiveHour: &input.Window{
		UsedPercentage: 40,
		ResetsAt:       now.Add(3 * time.Hour).Unix(),
	}}
	out, vis := w.Render(usageCtx(now, rl))
	if !vis || !strings.Contains(out, "40%") {
		t.Errorf("got %q", out)
	}
	if !strings.Contains(out, "3h") {
		t.Errorf("expected 3h countdown in %q", out)
	}
}

func TestUsage5hPaceOver(t *testing.T) {
	w := &Usage5h{}
	now := time.Unix(1_000_000, 0)
	rl := &input.RateLimits{FiveHour: &input.Window{
		UsedPercentage: 60,
		ResetsAt:       now.Add(3 * time.Hour).Unix(),
	}}
	out, _ := w.Render(usageCtx(now, rl))
	if !strings.Contains(out, "⇡") {
		t.Errorf("expected ⇡ over-consume marker in %q", out)
	}
}

func TestUsage5hPaceUnder(t *testing.T) {
	w := &Usage5h{}
	now := time.Unix(1_000_000, 0)
	rl := &input.RateLimits{FiveHour: &input.Window{
		UsedPercentage: 30,
		ResetsAt:       now.Add(1 * time.Hour).Unix(),
	}}
	out, _ := w.Render(usageCtx(now, rl))
	if !strings.Contains(out, "⇣") {
		t.Errorf("expected ⇣ headroom marker in %q", out)
	}
}

func TestUsage5hHidesWhenAbsent(t *testing.T) {
	if _, vis := (&Usage5h{}).Render(&Context{Cfg: config.Config{BarWidth: 4}}); vis {
		t.Errorf("expected hidden")
	}
}

func TestUsage7dRespectsThreshold(t *testing.T) {
	w := &Usage7d{}
	now := time.Unix(1_000_000, 0)
	rl := &input.RateLimits{SevenDay: &input.Window{
		UsedPercentage: 40,
		ResetsAt:       now.Add(6 * 24 * time.Hour).Unix(),
	}}
	if _, vis := w.Render(usageCtx(now, rl)); vis {
		t.Errorf("expected hidden when below threshold")
	}
	rl.SevenDay.UsedPercentage = 60
	if _, vis := w.Render(usageCtx(now, rl)); !vis {
		t.Errorf("expected visible when above threshold")
	}
}
