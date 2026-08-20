package widgets

import (
	"fmt"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const cacheGlyph = "\uf1c0 " // nf-fa-database

// Cache renders the prompt-cache accounting the pi cache-optimizer extension
// keeps for the active model: one figure, the share of input tokens that came
// back from the provider's cache.
//
// It was four fields once -- a bar, a cached/total token pair and a
// hit/total request ratio alongside the percentage. The first two were the
// percentage restated (the bar is pct/100; the pair is the fraction it
// divides). The request ratio was not, but a second number on a status line
// has to earn the width against the glance it costs, and this one did not.
//
// The sidecar totals are per day and per model, not per session, so this is
// "today, on this model" and it resets at midnight.
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
	return spans, true
}
