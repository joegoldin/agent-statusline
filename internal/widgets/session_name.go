package widgets

import "github.com/joegoldin/agent-statusline/internal/render"

const sessionNameGlyph = " " // nf-fa-tag

type SessionName struct{}

func (SessionName) Name() string { return "sessionName" }

func (SessionName) Render(ctx *Context) (string, bool) {
	spans, ok := SessionName{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (SessionName) RenderSpans(ctx *Context) (render.Spans, bool) {
	name := ctx.Status.SessionName
	if name == "" {
		return nil, false
	}
	return render.Spans{render.Text(render.IntentDim, sessionNameGlyph+name)}, true
}
