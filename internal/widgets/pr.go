package widgets

import (
	"fmt"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const prGlyph = " " // nf-cod-git_pull_request

type PR struct{}

func (PR) Name() string { return "pr" }

func (PR) Render(ctx *Context) (string, bool) {
	p := ctx.Status.PR
	if p == nil || p.Number == 0 {
		return "", false
	}
	text := fmt.Sprintf("%s#%d %s", prGlyph, p.Number, p.ReviewState)
	text = render.Cyan(text)
	if p.URL != "" {
		return render.Hyperlink(p.URL, text), true
	}
	return text, true
}
