package widgets

import "github.com/joegoldin/agent-statusline/internal/render"

const sessionNameGlyph = " " // nf-fa-tag

type SessionName struct{}

func (SessionName) Name() string { return "sessionName" }

func (SessionName) Render(ctx *Context) (string, bool) {
	name := ctx.Status.SessionName
	if name == "" {
		return "", false
	}
	return render.Dim(sessionNameGlyph + name), true
}
