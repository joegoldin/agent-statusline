package widgets

import (
	"fmt"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const gitGlyph = " " // nf-dev-git_branch

type Git struct{}

func (Git) Name() string { return "git" }

func (Git) Render(ctx *Context) (string, bool) {
	spans, ok := Git{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (Git) RenderSpans(ctx *Context) (render.Spans, bool) {
	g := ctx.Git()
	if g == nil {
		return nil, false
	}
	var label string
	switch {
	case g.Detached && g.SHA != "":
		short := g.SHA
		if len(short) > 7 {
			short = short[:7]
		}
		label = short
	case g.Branch != "":
		label = g.Branch
	default:
		return nil, false
	}
	if g.Dirty {
		label += "*"
	}
	spans := render.Spans{render.Text(render.IntentOK, gitGlyph+label)}
	// The separators are spans of their own rather than part of the dim runs:
	// under ANSI the concatenation is identical either way, but pi needs the
	// colour boundaries to survive to the renderer.
	add := func(s string) {
		spans = append(spans,
			render.Text(render.IntentText, " "),
			render.Text(render.IntentDim, s))
	}
	if g.Ahead > 0 {
		add(fmt.Sprintf("↑%d", g.Ahead))
	}
	if g.Behind > 0 {
		add(fmt.Sprintf("↓%d", g.Behind))
	}
	if wt := ctx.Status.Workspace.GitWorktree; wt != "" {
		// U+E5FB inside the brackets to signal "worktree". Pinned by
		// TestGitWidgetWorktreeGlyphIsPinned: no golden covers this branch,
		// because the e2e harness runs outside a repo and hides the widget.
		add("[ " + wt + "]")
	}
	return spans, true
}
