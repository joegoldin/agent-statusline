package widgets

import "github.com/joegoldin/agent-statusline/internal/render"

const effortGlyph = " " // nf-fa-brain

type Effort struct{}

func (Effort) Name() string { return "effort" }

func (Effort) Render(ctx *Context) (string, bool) {
	e := ctx.Status.Effort
	if e == nil || e.Level == "" {
		return "", false
	}
	return render.Magenta(effortGlyph + e.Level), true
}
