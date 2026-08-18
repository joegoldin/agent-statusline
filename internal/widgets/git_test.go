package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/gitcache"
	"github.com/joegoldin/agent-statusline/internal/input"
)

func gitCtx(g *gitcache.Git, worktree string) *Context {
	return &Context{
		Status:      input.Status{Workspace: input.Workspace{GitWorktree: worktree}},
		GitProvider: func() *gitcache.Git { return g },
	}
}

func TestGitWidgetRendersBranch(t *testing.T) {
	w := &Git{}
	out, vis := w.Render(gitCtx(&gitcache.Git{Branch: "main"}, ""))
	if !vis || !strings.Contains(out, "main") {
		t.Errorf("got %q", out)
	}
}

func TestGitWidgetShowsDirtyAndAheadBehind(t *testing.T) {
	w := &Git{}
	g := &gitcache.Git{Branch: "main", Dirty: true, Ahead: 2, Behind: 1}
	out, _ := w.Render(gitCtx(g, ""))
	if !strings.Contains(out, "*") {
		t.Errorf("expected dirty marker in %q", out)
	}
	if !strings.Contains(out, "↑2") {
		t.Errorf("expected ↑2 in %q", out)
	}
	if !strings.Contains(out, "↓1") {
		t.Errorf("expected ↓1 in %q", out)
	}
}

func TestGitWidgetAppendsWorktree(t *testing.T) {
	w := &Git{}
	out, _ := w.Render(gitCtx(&gitcache.Git{Branch: "main"}, "feature-xyz"))
	if !strings.Contains(out, "feature-xyz]") {
		t.Errorf("expected worktree badge in %q", out)
	}
	if !strings.Contains(out, "[") {
		t.Errorf("expected opening bracket in %q", out)
	}
}

func TestGitWidgetDetachedShowsSHA(t *testing.T) {
	w := &Git{}
	g := &gitcache.Git{Detached: true, SHA: "abcdef1234567890"}
	out, vis := w.Render(gitCtx(g, ""))
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "abcdef1") {
		t.Errorf("expected short SHA in %q", out)
	}
}

func TestGitWidgetHidesWhenNoRepo(t *testing.T) {
	w := &Git{}
	if _, vis := w.Render(&Context{GitProvider: func() *gitcache.Git { return nil }}); vis {
		t.Errorf("expected hidden when no git")
	}
}
