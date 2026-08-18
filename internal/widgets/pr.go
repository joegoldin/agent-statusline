package widgets

import (
	"fmt"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const prGlyph = " " // nf-cod-git_pull_request

type PR struct{}

func (PR) Name() string { return "pr" }

func (PR) Render(ctx *Context) (string, bool) {
	spans, ok := PR{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (PR) RenderSpans(ctx *Context) (render.Spans, bool) {
	p := ctx.Status.PR
	if p == nil || p.Number == 0 {
		return nil, false
	}
	text := fmt.Sprintf("%s#%d %s", prGlyph, p.Number, p.ReviewState)
	if p.URL != "" {
		return render.Spans{render.Link(render.IntentAccent, p.URL, text)}, true
	}
	return render.Spans{render.Text(render.IntentAccent, text)}, true
}
