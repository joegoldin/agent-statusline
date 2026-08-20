package widgets

import (
	"fmt"
	"strconv"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const cacheGlyph = "\uf1c0 " // nf-fa-database

// Cache renders the prompt-cache accounting the pi cache-optimizer extension
// keeps for the active model: what share of input tokens came back from the
// provider's cache, and how many requests hit it at all. Two figures, because
// they measure different things -- a session can cache 93% of its tokens while
// missing on 4 of 82 calls, since the misses are the small ones.
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
	// No bar and no token pair, at any width. Both were the percentage a second
	// and third time -- the bar is pct/100 by construction, and the token pair
	// is the very fraction the percentage divides -- so the row spent four
	// columns of figures saying one thing. The request ratio below is the only
	// number here that is not derivable from the percentage: it counts calls,
	// not tokens, which is why it can read 78/82 while the tokens read 93%.
	if t.TotalRequests > 0 {
		spans = append(spans,
			render.Text(render.IntentText, rowSeparator),
			render.Text(render.IntentMeta, strconv.Itoa(t.HitRequests)+"/"+strconv.Itoa(t.TotalRequests)))
	}
	return spans, true
}
