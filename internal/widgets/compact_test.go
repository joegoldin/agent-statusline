package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/input"
)

// Compact mode kicks in when Context.Width is set and below CompactWidth
// (default 70). Each widget shortens what it can to recover horizontal space.

func TestCompactThreshold(t *testing.T) {
	cases := []struct {
		width, threshold int
		want             bool
	}{
		{0, 0, false},   // unknown width never compacts
		{40, 0, true},   // default threshold (70) → compact
		{70, 0, false},  // exactly at threshold → not compact
		{120, 0, false}, // wide → not compact
		{60, 50, false}, // custom threshold below width → not compact
		{40, 50, true},  // custom threshold above width → compact
	}
	for _, tc := range cases {
		ctx := &Context{Width: tc.width, CompactWidth: tc.threshold}
		if got := ctx.Compact(); got != tc.want {
			t.Errorf("Compact(width=%d, threshold=%d) = %v, want %v", tc.width, tc.threshold, got, tc.want)
		}
	}
}

func TestContextBarDropsBarWhenCompact(t *testing.T) {
	pct := 47.0
	ctx := &Context{
		Width:  40,
		Status: input.Status{ContextWindow: &input.ContextWindow{ContextWindowSize: 200_000, UsedPercentage: &pct}},
		Cfg:    config.Config{BarWidth: 10},
	}
	out, vis := (ContextBar{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if strings.Contains(out, "⣿") {
		t.Errorf("compact ContextBar should not render braille bar: %q", out)
	}
	if !strings.Contains(out, "47%") {
		t.Errorf("compact ContextBar should still show percent: %q", out)
	}
}

func TestUsage5hDropsBarWhenCompact(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	rl := &input.RateLimits{FiveHour: &input.Window{
		UsedPercentage: 40,
		ResetsAt:       now.Add(3 * time.Hour).Unix(),
	}}
	ctx := &Context{
		Width:  40,
		Status: input.Status{RateLimits: rl},
		Cfg:    config.Config{BarWidth: 10},
		Now:    now,
	}
	out, vis := (Usage5h{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	for _, glyph := range []string{"█", "▏", "▎", "▍", "▌", "▋", "▊", "▉"} {
		if strings.Contains(out, glyph) {
			t.Errorf("compact Usage5h should not render block bar (%s in %q)", glyph, out)
		}
	}
	if !strings.Contains(out, "40%") || !strings.Contains(out, "3h") {
		t.Errorf("compact Usage5h should still show percent + countdown: %q", out)
	}
}

func TestUsage7dDropsBarWhenCompact(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	rl := &input.RateLimits{SevenDay: &input.Window{
		UsedPercentage: 65,
		ResetsAt:       now.Add(5 * 24 * time.Hour).Unix(),
	}}
	ctx := &Context{
		Width:  40,
		Status: input.Status{RateLimits: rl},
		Cfg:    config.Config{BarWidth: 10, SevenDayThreshold: 50},
		Now:    now,
	}
	out, vis := (Usage7d{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if strings.Contains(out, "━") {
		t.Errorf("compact Usage7d should not render line bar: %q", out)
	}
	if !strings.Contains(out, "65%") {
		t.Errorf("compact Usage7d should still show percent: %q", out)
	}
}

func TestTokensShortensSuffixWhenCompact(t *testing.T) {
	tokens := 156_300
	pct := 16.0
	ctx := &Context{
		Width: 40,
		Status: input.Status{
			ContextWindow: &input.ContextWindow{
				ContextWindowSize: 1_000_000,
				TotalInputTokens:  tokens,
				UsedPercentage:    &pct,
			},
		},
	}
	out, vis := (Tokens{}).Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "tok") || strings.Contains(out, "tokens") {
		t.Errorf("compact Tokens should use \"tok\" not \"tokens\": %q", out)
	}
}

func TestTokensKeepsLongSuffixWhenWide(t *testing.T) {
	tokens := 156_300
	pct := 16.0
	ctx := &Context{
		Width: 120,
		Status: input.Status{
			ContextWindow: &input.ContextWindow{
				ContextWindowSize: 1_000_000,
				TotalInputTokens:  tokens,
				UsedPercentage:    &pct,
			},
		},
	}
	out, _ := (Tokens{}).Render(ctx)
	if !strings.Contains(out, "tokens") {
		t.Errorf("wide Tokens should use \"tokens\": %q", out)
	}
}
