package widgets

import (
	"regexp"
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const modelGlyph = " " // nf-fa-cogs

// Model renders the model name with the trailing "(1M context)" suffix
// stripped and (when present) the current reasoning effort appended inline.
// Rendered effort follows the model name with a single space, matching the
// "Opus 4.7 xhigh" look.
type Model struct{}

func (Model) Name() string { return "model" }

var modelSuffixRE = regexp.MustCompile(`\s*\(1M context\)\s*$`)

func (Model) Render(ctx *Context) (string, bool) {
	spans, ok := Model{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Model) RenderSpans(ctx *Context) (render.Spans, bool) {
	name := strings.TrimSpace(ctx.Status.Model.DisplayName)
	if name == "" {
		return nil, false
	}
	name = modelSuffixRE.ReplaceAllString(name, "")
	// Distinguish the 1M-context variant of a model — there are two
	// Opus 4.7 SKUs that share the same display_name. The reliable
	// signal is context_window.context_window_size; the [1m] suffix on
	// model.id is checked as a fallback but real transcripts show plain
	// "claude-opus-4-7" without it.
	if is1MContext(ctx) {
		name += " 1M"
	}
	out := modelGlyph + name
	if e := ctx.Status.Effort; e != nil && e.Level != "" {
		out += " " + e.Level
	}
	return render.Spans{render.Text(render.IntentAccent, out)}, true
}

// is1MContext returns true when the active model is a 1M-context variant.
// Primary signal: context_window_size ≥ 1_000_000. Fallback: [1m] tag on
// model.id (some legacy/synthetic JSON uses that).
func is1MContext(ctx *Context) bool {
	if cw := ctx.Status.ContextWindow; cw != nil && cw.ContextWindowSize >= 1_000_000 {
		return true
	}
	return strings.Contains(strings.ToLower(ctx.Status.Model.ID), "[1m]")
}
