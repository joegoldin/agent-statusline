package widgets

import (
	"fmt"
	"strconv"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const cacheGlyph = "\uf1c0 " // nf-fa-database

// Cache renders the prompt-cache accounting the pi cache-optimizer extension
// keeps for the active model: what share of input tokens came back from the
// provider's cache, and how many requests hit it at all.
type Cache struct{}

func (Cache) Name() string { return "cache" }

func (Cache) Render(ctx *Context) (string, bool) {
	spans, ok := Cache{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Cache) RenderSpans(ctx *Context) (render.Spans, bool) {
	t, ok := ctx.CacheStats().Lookup(ctx.Status.Model.Provider, ctx.Status.Model.ID)
	// A model the sidecar has never recorded, or has recorded with no traffic,
	// gets no row: 0.0% would read as a cache that is failing rather than one
	// that has not been asked anything yet.
	if !ok || t.TotalInputTokens <= 0 {
		return nil, false
	}
	pct := float64(t.CachedInputTokens) / float64(t.TotalInputTokens) * 100
	// The threshold palettes escalate as their input grows, which is backwards
	// for a hit rate: 100% cached is the good end. Feed them the miss rate.
	intent := render.ThresholdIntent(100 - pct)
	spans := render.Spans{
		render.Text(intent, fmt.Sprintf("%scache %.1f%%", cacheGlyph, pct)),
	}
	// Compact terminal: the bar and the token pair are the widest parts and
	// say the same thing the percentage already did.
	if !ctx.Compact() {
		width := ctx.Cfg.BarWidth
		if width <= 0 {
			width = 10
		}
		spans = append(spans,
			render.Text(render.IntentText, rowSeparator),
			render.Bar(pct/100, width, render.BarBraille),
			render.Text(render.IntentText, rowSeparator),
			render.Text(render.IntentDim, humanTokens(t.CachedInputTokens)+"/"+humanTokens(t.TotalInputTokens)))
	}
	if t.TotalRequests > 0 {
		spans = append(spans,
			render.Text(render.IntentText, rowSeparator),
			render.Text(render.IntentMeta, strconv.Itoa(t.HitRequests)+"/"+strconv.Itoa(t.TotalRequests)))
	}
	return spans, true
}

// humanTokens compacts a token count to three significant digits, so the
// cached/total pair reads as a ratio you can check by eye. It is deliberately
// not formatTokens: that one's single decimal ("1.7M") is pinned by the Claude
// golden files and loses the digit that makes the comparison legible.
func humanTokens(n int) string {
	if n < 1_000 {
		return strconv.Itoa(n)
	}
	// Rounded thousands, unless the rounding carries into millions.
	if k := (n + 500) / 1_000; k < 1_000 {
		return strconv.Itoa(k) + "k"
	}
	m := float64(n) / 1_000_000
	switch {
	case m >= 100:
		return fmt.Sprintf("%.0fM", m)
	case m >= 10:
		return fmt.Sprintf("%.1fM", m)
	default:
		return fmt.Sprintf("%.2fM", m)
	}
}
