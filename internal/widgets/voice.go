package widgets

import (
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const voiceGlyph = "" // nf-fa-microphone

type Voice struct{}

func (Voice) Name() string { return "voice" }

func (Voice) Render(ctx *Context) (string, bool) {
	cfg := ctx.Voice()
	if cfg == nil || !cfg.Enabled {
		return "", false
	}
	out := voiceGlyph
	if mode := strings.TrimSpace(cfg.Mode); mode != "" {
		out += " " + mode
	}
	return render.Magenta(out), true
}
