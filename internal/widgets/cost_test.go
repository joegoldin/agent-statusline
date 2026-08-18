package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestCostHidesForMaxSubscriberInsideLimits(t *testing.T) {
	w := &Cost{}
	ctx := &Context{Status: input.Status{
		Cost: &input.Cost{TotalCostUSD: 1.42},
		RateLimits: &input.RateLimits{
			FiveHour: &input.Window{UsedPercentage: 40},
			SevenDay: &input.Window{UsedPercentage: 65},
		},
	}}
	if _, vis := w.Render(ctx); vis {
		t.Errorf("expected hidden when inside Max plan limits")
	}
}

func TestCostShowsAtFiveHourOverage(t *testing.T) {
	w := &Cost{}
	ctx := &Context{Status: input.Status{
		Cost: &input.Cost{TotalCostUSD: 1.42},
		RateLimits: &input.RateLimits{
			FiveHour: &input.Window{UsedPercentage: 100},
			SevenDay: &input.Window{UsedPercentage: 65},
		},
	}}
	out, vis := w.Render(ctx)
	if !vis || !strings.Contains(out, "$1.42") {
		t.Errorf("got %q", out)
	}
}

func TestCostShowsAtSevenDayOverage(t *testing.T) {
	w := &Cost{}
	ctx := &Context{Status: input.Status{
		Cost: &input.Cost{TotalCostUSD: 1.42},
		RateLimits: &input.RateLimits{
			FiveHour: &input.Window{UsedPercentage: 40},
			SevenDay: &input.Window{UsedPercentage: 100},
		},
	}}
	out, vis := w.Render(ctx)
	if !vis || !strings.Contains(out, "$1.42") {
		t.Errorf("got %q", out)
	}
}

func TestCostHidesWhenNoRateLimits(t *testing.T) {
	// rate_limits is nil on resumed Max sessions until the first API
	// response populates it. We deliberately hide cost in that case rather
	// than flash a misleading dollar figure during session startup. Non-
	// subscribers who actually want to see cost can override via Nix.
	w := &Cost{}
	ctx := &Context{Status: input.Status{Cost: &input.Cost{TotalCostUSD: 1.42}}}
	if _, vis := w.Render(ctx); vis {
		t.Errorf("expected hidden when rate_limits is absent")
	}
}

func TestCostHidesWhenAbsent(t *testing.T) {
	if _, vis := (&Cost{}).Render(&Context{}); vis {
		t.Errorf("expected hidden")
	}
}

func TestCostHidesAtZero(t *testing.T) {
	w := &Cost{}
	ctx := &Context{Status: input.Status{Cost: &input.Cost{TotalCostUSD: 0}}}
	if _, vis := w.Render(ctx); vis {
		t.Errorf("expected hidden at 0")
	}
}

func TestCostAlwaysVisibleInPiMode(t *testing.T) {
	ctx := &Context{
		Mode: input.ModePi,
		Status: input.Status{
			Cost: &input.Cost{TotalCostUSD: 1.23},
			// No RateLimits: exactly the non-Anthropic case.
		},
	}
	text, visible := (Cost{}).Render(ctx)
	if !visible {
		t.Fatal("cost widget hidden in pi mode; want visible")
	}
	if !strings.Contains(text, "1.23") {
		t.Errorf("cost text = %q, want it to contain 1.23", text)
	}
}

func TestCostStillGatedInClaudeMode(t *testing.T) {
	ctx := &Context{
		Mode: input.ModeClaude,
		Status: input.Status{
			Cost: &input.Cost{TotalCostUSD: 1.23},
			RateLimits: &input.RateLimits{
				FiveHour: &input.Window{UsedPercentage: 10},
			},
		},
	}
	if _, visible := (Cost{}).Render(ctx); visible {
		t.Error("cost widget visible in claude mode below overage; want hidden")
	}
}

func TestCostHiddenWhenZeroInPiMode(t *testing.T) {
	ctx := &Context{
		Mode:   input.ModePi,
		Status: input.Status{Cost: &input.Cost{TotalCostUSD: 0}},
	}
	if _, visible := (Cost{}).Render(ctx); visible {
		t.Error("cost widget visible with zero cost; want hidden")
	}
}
