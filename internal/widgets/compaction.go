package widgets

import (
	"fmt"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const compactionGlyph = " " // nf-fa-compress

type Compaction struct{}

func (Compaction) Name() string { return "compaction" }

func (Compaction) Render(ctx *Context) (string, bool) {
	spans, ok := Compaction{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Compaction) RenderSpans(ctx *Context) (render.Spans, bool) {
	n := ctx.Compactions()
	if n <= 0 {
		return nil, false
	}
	return render.Spans{render.Text(render.IntentDim, fmt.Sprintf("%s%dc", compactionGlyph, n))}, true
}
