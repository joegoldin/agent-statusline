package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/input"
)

func ctxPctStatus(pct float64) input.Status {
	return input.Status{
		ContextWindow: &input.ContextWindow{
			ContextWindowSize: 200_000,
			UsedPercentage:    &pct,
		},
	}
}

func TestContextBarUsesPercentage(t *testing.T) {
	w := &ContextBar{}
	ctx := &Context{Status: ctxPctStatus(47), Cfg: config.Config{BarWidth: 8}}
	out, vis := w.Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "47%") {
		t.Errorf("expected 47%% in %q", out)
	}
	if !strings.Contains(out, "⣿") {
		t.Errorf("expected braille filled bar in %q", out)
	}
}

func TestContextBarHidesWhenMissing(t *testing.T) {
	w := &ContextBar{}
	if _, vis := w.Render(&Context{}); vis {
		t.Errorf("expected hidden")
	}
}

func TestContextBarInfersOneMillionContext(t *testing.T) {
	w := &ContextBar{}
	status := input.Status{
		Model: input.Model{ID: "claude-opus-4-7[1m]"},
		ContextWindow: &input.ContextWindow{
			TotalInputTokens: 250_000,
		},
	}
	ctx := &Context{Status: status, Cfg: config.Config{BarWidth: 8}}
	out, vis := w.Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "25%") {
		t.Errorf("expected 25%% from 1M inference, got %q", out)
	}
}
