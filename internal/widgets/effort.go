package widgets

import "github.com/joegoldin/agent-statusline/internal/render"

const effortGlyph = " " // nf-fa-brain

type Effort struct{}

func (Effort) Name() string { return "effort" }

func (Effort) Render(ctx *Context) (string, bool) {
	spans, ok := Effort{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Effort) RenderSpans(ctx *Context) (render.Spans, bool) {
	e := ctx.Status.Effort
	if e == nil || e.Level == "" {
		return nil, false
	}
	return render.Spans{render.Text(render.IntentMeta, effortGlyph+e.Level)}, true
}
