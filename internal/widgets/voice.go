package widgets

import (
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const voiceGlyph = "" // nf-fa-microphone

type Voice struct{}

func (Voice) Name() string { return "voice" }

func (Voice) Render(ctx *Context) (string, bool) {
	spans, ok := Voice{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Voice) RenderSpans(ctx *Context) (render.Spans, bool) {
	cfg := ctx.Voice()
	if cfg == nil || !cfg.Enabled {
		return nil, false
	}
	out := voiceGlyph
	if mode := strings.TrimSpace(cfg.Mode); mode != "" {
		out += " " + mode
	}
	return render.Spans{render.Text(render.IntentMeta, out)}, true
}
