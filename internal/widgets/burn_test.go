package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/transcript"
)

func burnCtx(now time.Time, reqs []transcript.Request, usedPct float64) *Context {
	pct := usedPct
	return &Context{
		Status: input.Status{
			Model: input.Model{ID: "claude-opus-4-7"},
			ContextWindow: &input.ContextWindow{
				ContextWindowSize: 200_000,
				UsedPercentage:    &pct,
			},
		},
		Cfg: config.Config{TranscriptWindowSeconds: 60},
		Now: now,
		TranscriptProvider: func() *transcript.Entries {
			return &transcript.Entries{Requests: reqs}
		},
	}
}

func TestBurnRendersPercentPerMinute(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	// 5000 new tokens within τ=60s on a 200k context → ~83 tok/s EMA-ish
	// → 5000/min → 2.5%/min
	reqs := []transcript.Request{
		{Timestamp: now.Add(-1 * time.Second), InputTokens: 4000, OutputTokens: 1000},
	}
	out, vis := (&BurnRate{}).Render(burnCtx(now, reqs, 50))
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "%/m") {
		t.Errorf("expected %%/m unit in %q", out)
	}
	if !strings.Contains(out, "ETA") {
		t.Errorf("expected ETA in %q", out)
	}
}

func TestBurnExcludesCacheReadsFromRate(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	// Huge cache read should NOT show as a high burn.
	reqs := []transcript.Request{
		{Timestamp: now.Add(-1 * time.Second), CacheRead: 500_000, InputTokens: 100, OutputTokens: 50},
	}
	out, vis := (&BurnRate{}).Render(burnCtx(now, reqs, 50))
	if !vis {
		t.Fatal("expected visible")
	}
	// 150 new tokens at ~0.98 weight / 60s ≈ 2.4 tok/s → 144 tok/min → 0.07%/m on 200k context.
	// Definitely not double-digit percent.
	if strings.Contains(out, "10%/m") || strings.Contains(out, "20%/m") || strings.Contains(out, "50%/m") {
		t.Errorf("cache read leaked into displayed rate: %q", out)
	}
}

func TestBurnHidesWithoutTranscript(t *testing.T) {
	if _, vis := (&BurnRate{}).Render(&Context{}); vis {
		t.Errorf("expected hidden")
	}
}

func TestBurnETARedAtNearFull(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	// Heavy burn, near-full context → ETA red.
	reqs := []transcript.Request{
		{Timestamp: now.Add(-1 * time.Second), InputTokens: 100_000},
	}
	out, _ := (&BurnRate{}).Render(burnCtx(now, reqs, 95))
	if !strings.Contains(out, "ETA") {
		t.Fatalf("expected ETA in %q", out)
	}
	// Red SGR is \x1b[31m
	if !strings.Contains(out, "\x1b[31m") {
		t.Errorf("expected red ETA in %q", out)
	}
}

func TestBurnRateStableWithin30sWindow(t *testing.T) {
	bucket := time.Unix(1_000_020, 0).Truncate(30 * time.Second)
	reqs := []transcript.Request{
		{Timestamp: bucket.Add(-5 * time.Second), InputTokens: 4000, OutputTokens: 1000},
	}
	// Two renders one second apart (but in the same 30s bucket) must produce
	// identical output — otherwise the rate/ETA jitters every 1s refresh.
	a, _ := (&BurnRate{}).Render(burnCtx(bucket.Add(1*time.Second), reqs, 50))
	b, _ := (&BurnRate{}).Render(burnCtx(bucket.Add(29*time.Second), reqs, 50))
	if a != b {
		t.Errorf("burn output jittered within a 30s window:\n a=%q\n b=%q", a, b)
	}
}

func TestBurnRateUpdatesOnNewRequest(t *testing.T) {
	bucket := time.Unix(1_000_020, 0).Truncate(30 * time.Second)
	now := bucket.Add(2 * time.Second)
	reqsA := []transcript.Request{
		{Timestamp: bucket.Add(-5 * time.Second), InputTokens: 2000, OutputTokens: 500},
	}
	reqsB := append(append([]transcript.Request{}, reqsA...),
		transcript.Request{Timestamp: bucket.Add(-1 * time.Second), InputTokens: 8000, OutputTokens: 2000})
	a, _ := (&BurnRate{}).Render(burnCtx(now, reqsA, 50))
	b, _ := (&BurnRate{}).Render(burnCtx(now, reqsB, 50))
	if a == b {
		t.Errorf("burn rate should update when a new request arrives, got %q both times", a)
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{15.4, "15%/m"},
		{2.43, "2.4%/m"},
		{0.123, "0.12%/m"},
		{0.01, "0.01%/m"}, // smallest rendered; below this the widget hides
	}
	for _, tc := range tests {
		if got := formatRate(tc.in); got != tc.want {
			t.Errorf("formatRate(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
