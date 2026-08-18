package transcript

import (
	"math"
	"time"
)

// newTokens returns the count of tokens *added to context* by a single
// request. cache_read_input_tokens are excluded — those tokens are already
// in context (a cached prompt being re-read on each turn), so counting
// them would massively inflate the burn rate.
func newTokens(r Request) int {
	return r.InputTokens + r.CacheCreate + r.OutputTokens
}

// TokensInWindow returns the count of *new* tokens added to the context
// window for requests within [now-window, now]. Subagent activity is
// included. Cache reads are excluded (see newTokens).
func TokensInWindow(reqs []Request, now time.Time, window time.Duration) int {
	cutoff := now.Add(-window)
	total := 0
	for _, r := range reqs {
		if r.Timestamp.Before(cutoff) || r.Timestamp.After(now) {
			continue
		}
		total += newTokens(r)
	}
	return total
}

// TokensPerSecondEMA returns the time-weighted exponential moving average
// rate of new tokens (in tokens/sec) at `now`, with decay constant `tau`.
// Each request contributes tokens_i * exp(-(now - t_i)/tau), normalized
// by tau so the result is a rate. Returns 0 if no recent activity.
//
// Compared to a flat rolling window, EMA has no "cliff": a request 60s
// ago carries weight ~0.37 (with tau=60s), 180s ago ~0.05, smoothly
// fading instead of vanishing the moment it falls outside the window.
func TokensPerSecondEMA(reqs []Request, now time.Time, tau time.Duration) float64 {
	if tau <= 0 || len(reqs) == 0 {
		return 0
	}
	tauSec := tau.Seconds()
	weightedSum := 0.0
	for _, r := range reqs {
		dt := now.Sub(r.Timestamp).Seconds()
		if dt < 0 {
			continue
		}
		w := math.Exp(-dt / tauSec)
		weightedSum += float64(newTokens(r)) * w
	}
	return weightedSum / tauSec
}

// TokensPerSecond is retained as a flat-window alternative (and used by
// some legacy tests). New callers should prefer TokensPerSecondEMA.
func TokensPerSecond(reqs []Request, now time.Time, window time.Duration) float64 {
	tokens := TokensInWindow(reqs, now, window)
	if tokens == 0 || window <= 0 {
		return 0
	}
	return float64(tokens) / window.Seconds()
}
