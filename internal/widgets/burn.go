package widgets

import (
	"fmt"
	"strings"
	"time"

	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/render"
	"github.com/joegoldin/agent-statusline/internal/transcript"
)

// burnGlyph — nf-fa-bolt (U+F0E7). Width-1 unlike the ⚡ emoji (which is
// width-2 in most terminals and produced visible padding after itself).
const burnGlyph = ""

// burnBucket quantizes the time fed to the EMA so the rate/ETA only re-steps
// at 30s boundaries instead of drifting on every 1s refresh. A new request
// still updates the display immediately (it changes the EMA's inputs, not
// just the elapsed time).
const burnBucket = 30 * time.Second

// burnNow rounds now UP to the next burnBucket boundary. Rounding up (rather
// than truncating down) keeps the quantized time at or after `now`, so a
// just-arrived request is never treated as being in the future and dropped
// from the EMA.
func burnNow(now time.Time) time.Time {
	return now.Truncate(burnBucket).Add(burnBucket)
}

// BurnRate displays the rate at which new tokens are entering the context
// window, as percentage-per-minute (intuitive: 100% / rate ≈ minutes to
// full). Computed via a time-weighted EMA so a single spike doesn't fall
// off a cliff. Cache-read tokens are excluded — they don't grow the context.
type BurnRate struct{}

func (BurnRate) Name() string { return "burnRate" }

func (BurnRate) Render(ctx *Context) (string, bool) {
	entries := ctx.Transcript()
	if entries == nil || len(entries.Requests) == 0 {
		return "", false
	}
	size, ok := contextWindowSize(ctx.Status)
	if !ok || size <= 0 {
		return "", false
	}
	tau := time.Duration(ctx.Cfg.TranscriptWindowSeconds) * time.Second
	if tau <= 0 {
		tau = 60 * time.Second
	}
	tps := transcript.TokensPerSecondEMA(entries.Requests, burnNow(ctx.Now), tau)
	if tps <= 0 {
		return "", false
	}
	pctPerMin := tps * 60 / float64(size) * 100
	// Below ~0.01 %/m the rate isn't actionable — most likely an idle
	// session showing stale tail tokens. Hide the whole widget rather than
	// invent obscure units. The burn-rate column will simply disappear.
	if pctPerMin < 0.01 {
		return "", false
	}
	rateStr := formatRate(pctPerMin)
	left := fmt.Sprintf("%s %s", burnGlyph, rateStr)
	// Compact terminal: drop the ETA tail so only the rate remains.
	if ctx.Compact() {
		return render.Magenta(left), true
	}

	pct, _ := contextPercent(ctx.Status)
	eta := etaToFull(size, pct, tps)
	// Only surface ETA when the projection is short enough to be meaningful.
	// Anything beyond ~24h is closer to "indefinite" than a useful number.
	const etaCap = 24 * time.Hour
	if eta <= 0 || eta > etaCap {
		return render.Magenta(left), true
	}
	etaStr := fmt.Sprintf("ETA %s", formatDuration(eta))
	if eta < 15*time.Minute {
		etaStr = render.Red(etaStr)
	} else {
		etaStr = render.Dim(etaStr)
	}
	return render.Magenta(left) + " " + etaStr, true
}

// contextWindowSize returns the effective context window size, with the
// [1m] model-id fallback that matches contextPercent in context_bar.go.
func contextWindowSize(s input.Status) (int, bool) {
	cw := s.ContextWindow
	if cw == nil {
		return 0, false
	}
	if cw.ContextWindowSize > 0 {
		return cw.ContextWindowSize, true
	}
	if strings.Contains(strings.ToLower(s.Model.ID), "[1m]") {
		return 1_000_000, true
	}
	return 200_000, true
}

// etaToFull projects when context hits 100% given a current new-token rate.
func etaToFull(size int, usedPct float64, tps float64) time.Duration {
	if usedPct >= 100 || tps <= 0 || size <= 0 {
		return 0
	}
	remainingTokens := float64(size) * (100 - usedPct) / 100
	secs := remainingTokens / tps
	return time.Duration(secs * float64(time.Second))
}

// formatRate renders the burn rate as a compact %/min value.
//
//   - ≥10 %/m: integer ("12%/m")
//   - 0.5–10 %/m: one decimal ("2.4%/m")
//   - <0.5 %/m: two decimals ("0.08%/m")
//
// Callers should hide the widget when pctPerMin is below ~0.01 — the
// rate is too small to render meaningfully and ETA projections become
// absurd (hours/days for a quiet session).
func formatRate(pctPerMin float64) string {
	if pctPerMin >= 10 {
		return fmt.Sprintf("%d%%/m", int(pctPerMin+0.5))
	}
	if pctPerMin >= 0.5 {
		return fmt.Sprintf("%.1f%%/m", pctPerMin)
	}
	return fmt.Sprintf("%.2f%%/m", pctPerMin)
}

// formatTokens compacts large numbers (e.g. 1234 → "1.2k").
// Retained because the tokens widget uses it.
func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
