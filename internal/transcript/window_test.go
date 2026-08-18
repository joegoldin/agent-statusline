package transcript

import (
	"testing"
	"time"
)

func TestTokensInWindowExcludesCacheReads(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	reqs := []Request{
		{Timestamp: now.Add(-120 * time.Second), InputTokens: 1000, CacheRead: 50_000, OutputTokens: 200},
		{Timestamp: now.Add(-30 * time.Second), InputTokens: 500, CacheCreate: 200, CacheRead: 100_000, OutputTokens: 100},
		{Timestamp: now.Add(-5 * time.Second), InputTokens: 300, CacheRead: 80_000, OutputTokens: 50},
	}
	// Window covers the last two requests. New tokens only:
	// (500 + 200 + 100) + (300 + 50) = 800 + 350 = 1150
	tokens := TokensInWindow(reqs, now, 60*time.Second)
	if tokens != 1150 {
		t.Errorf("tokens = %d, want 1150 (cache reads excluded)", tokens)
	}
}

func TestTokensPerSecond(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	reqs := []Request{
		{Timestamp: now.Add(-30 * time.Second), InputTokens: 1000, OutputTokens: 200},
		{Timestamp: now.Add(-10 * time.Second), InputTokens: 500, OutputTokens: 100},
	}
	tps := TokensPerSecond(reqs, now, 60*time.Second)
	if tps != 30.0 {
		t.Errorf("tps = %f, want 30.0", tps)
	}
}

func TestTokensPerSecondNoData(t *testing.T) {
	if tps := TokensPerSecond(nil, time.Now(), 60*time.Second); tps != 0 {
		t.Errorf("tps with no data = %f, want 0", tps)
	}
}

func TestTokensPerSecondEMAWeightsRecentHigher(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	tau := 60 * time.Second
	// Two identical bursts: one 5s ago, one 180s ago. The recent one
	// should dominate the EMA.
	reqs := []Request{
		{Timestamp: now.Add(-180 * time.Second), InputTokens: 6000},
		{Timestamp: now.Add(-5 * time.Second), InputTokens: 6000},
	}
	ema := TokensPerSecondEMA(reqs, now, tau)
	// Recent weight ≈ exp(-5/60) ≈ 0.920; old weight ≈ exp(-180/60) ≈ 0.050.
	// (6000*0.920 + 6000*0.050) / 60 = 97.0 tok/s
	if ema < 90 || ema > 110 {
		t.Errorf("EMA = %f, want ~97", ema)
	}
}

func TestTokensPerSecondEMAExcludesCacheReads(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	tau := 60 * time.Second
	// A huge cache read 1s ago should NOT inflate the rate.
	reqs := []Request{
		{Timestamp: now.Add(-1 * time.Second), CacheRead: 500_000, InputTokens: 100, OutputTokens: 50},
	}
	ema := TokensPerSecondEMA(reqs, now, tau)
	// Only 150 new tokens contribute; weight ~exp(-1/60) ≈ 0.983 → ~2.46 tok/s
	if ema > 10 {
		t.Errorf("EMA = %f, cache read leaked into rate", ema)
	}
}

func TestTokensPerSecondEMANoData(t *testing.T) {
	if ema := TokensPerSecondEMA(nil, time.Now(), 60*time.Second); ema != 0 {
		t.Errorf("EMA with no data = %f, want 0", ema)
	}
}
