package widgets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/cachestats"
	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/render"
)

const cacheStatsFixture = `{
  "version": 6,
  "totalsByModel": {
    "openai-codex/gpt-5.6-sol": {
      "day": "2026-08-19",
      "totalRequests": 41,
      "hitRequests": 35,
      "cachedInputTokens": 1690000,
      "cacheWriteInputTokens": 0,
      "totalInputTokens": 2130000
    },
    "anthropic/claude-opus-4-8": {
      "day": "2026-08-19",
      "totalRequests": 0,
      "hitRequests": 0,
      "cachedInputTokens": 0,
      "cacheWriteInputTokens": 0,
      "totalInputTokens": 0
    }
  }
}`

// cacheCtx wires the widget the way main.go does — through cachestats.Load on a
// real file — so "no file" and "bad file" are exercised as the widget will
// actually meet them. An empty body writes no file at all.
func cacheCtx(t *testing.T, body, provider, id string) *Context {
	t.Helper()
	path := filepath.Join(t.TempDir(), cachestats.FileName)
	if body != "" {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Context{
		Cfg:    config.Defaults(),
		Status: input.Status{Model: input.Model{ID: id, Provider: provider}},
		CacheStatsProvider: func() *cachestats.Stats {
			s, err := cachestats.Load(path)
			if err != nil {
				return nil
			}
			return s
		},
	}
}

func TestCacheRender(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		provider string
		id       string
		visible  bool
		want     []string
	}{
		{
			name:     "active model",
			body:     cacheStatsFixture,
			provider: "openai-codex",
			id:       "gpt-5.6-sol",
			visible:  true,
			want:     []string{"cache", "79.3%"},
		},
		{
			name:     "file missing",
			provider: "openai-codex",
			id:       "gpt-5.6-sol",
		},
		{
			name:     "model key absent",
			body:     cacheStatsFixture,
			provider: "google",
			id:       "gemini-3-pro",
		},
		{
			name:     "malformed JSON",
			body:     `{"totalsByModel": {`,
			provider: "openai-codex",
			id:       "gpt-5.6-sol",
		},
		{
			name:     "model recorded but no traffic yet",
			body:     cacheStatsFixture,
			provider: "anthropic",
			id:       "claude-opus-4-8",
		},
		{
			name: "no provider on the model",
			body: cacheStatsFixture,
			id:   "gpt-5.6-sol",
		},
	}
	for _, tc := range tests {
		ctx := cacheCtx(t, tc.body, tc.provider, tc.id)
		out, vis := Cache{}.Render(ctx)
		if vis != tc.visible {
			t.Errorf("%s: visible = %v, want %v (out=%q)", tc.name, vis, tc.visible, out)
			continue
		}
		for _, want := range tc.want {
			if !strings.Contains(out, want) {
				t.Errorf("%s: %q missing from %q", tc.name, want, out)
			}
		}
	}
}

func TestCacheHidesWithoutAProvider(t *testing.T) {
	ctx := &Context{Cfg: config.Defaults(), Status: input.Status{Model: input.Model{ID: "gpt-5.6-sol", Provider: "openai-codex"}}}
	if _, vis := (Cache{}).Render(ctx); vis {
		t.Error("expected hidden when no cache-stats provider is wired")
	}
}

func TestCacheRowIsTheGlyphAndThePercent(t *testing.T) {
	// One figure, at every width. The bar and the cached/total token pair were
	// the percentage restated, and the hit/total request ratio was dropped
	// after it: a second number has to earn its width on a status line.
	ctx := cacheCtx(t, cacheStatsFixture, "openai-codex", "gpt-5.6-sol")
	for _, width := range []int{40, 80, 200} {
		ctx.Width = width
		spans, ok := Cache{}.RenderSpans(ctx)
		if !ok {
			t.Fatalf("width %d: expected visible", width)
		}
		for _, sp := range spans {
			if sp.Kind == "bar" {
				t.Errorf("width %d: cache row still emits a bar span", width)
			}
		}
		if got, want := render.StripANSI(spans.ANSI()), "\uf1c0 cache 79.3%"; got != want {
			t.Errorf("width %d: row = %q, want %q", width, got, want)
		}
	}
}

func TestCacheRowStartsWithItsGlyph(t *testing.T) {
	// The literal shipped as a bare space: the comment named nf-fa-database and
	// the string held only the separator, so the cache row drew one column in
	// from every other row with no icon where theirs sat. Asserted through
	// Render, because what went wrong is what the row looks like.
	ctx := cacheCtx(t, cacheStatsFixture, "openai-codex", "gpt-5.6-sol")
	out, ok := (Cache{}).Render(ctx)
	if !ok {
		t.Fatal("expected the cache row to render")
	}
	if !strings.HasPrefix(render.StripANSI(out), "\uf1c0 cache ") {
		t.Errorf("cache row = %q, want it to open with the nf-fa-database glyph", render.StripANSI(out))
	}
}
