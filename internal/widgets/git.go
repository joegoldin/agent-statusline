package widgets

import (
	"fmt"
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const gitGlyph = " " // nf-dev-git_branch

type Git struct{}

func (Git) Name() string { return "git" }

func (Git) Render(ctx *Context) (string, bool) {
	g := ctx.Git()
	if g == nil {
		return "", false
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
		return "", false
	}
	if g.Dirty {
		label += "*"
	}
	parts := []string{render.Green(gitGlyph + label)}
	if g.Ahead > 0 {
		parts = append(parts, render.Dim(fmt.Sprintf("↑%d", g.Ahead)))
	}
	if g.Behind > 0 {
		parts = append(parts, render.Dim(fmt.Sprintf("↓%d", g.Behind)))
	}
	if wt := ctx.Status.Workspace.GitWorktree; wt != "" {
		// nf-fa-tree (U+F1BB) inside the brackets to signal "worktree".
		parts = append(parts, render.Dim("[ "+wt+"]"))
	}
	return strings.Join(parts, " "), true
}
