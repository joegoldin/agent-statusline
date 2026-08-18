package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestCWDAbbreviatesHome(t *testing.T) {
	t.Setenv("HOME", "/home/joe")
	w := &CWD{}
	ctx := &Context{Status: input.Status{CWD: "/home/joe/projects/claude-nix"}}
	out, vis := w.Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if strings.Contains(out, "/home/joe") {
		t.Errorf("HOME not abbreviated: %q", out)
	}
	if !strings.Contains(out, "~") {
		t.Errorf("expected ~ abbreviation: %q", out)
	}
}

func TestCWDLastTwoSegments(t *testing.T) {
	t.Setenv("HOME", "/home/joe")
	w := &CWD{}
	ctx := &Context{Status: input.Status{CWD: "/home/joe/a/b/c/d/e"}}
	out, _ := w.Render(ctx)
	if !strings.Contains(out, "d/e") {
		t.Errorf("expected d/e in output: %q", out)
	}
	if strings.Contains(out, "a/b") {
		t.Errorf("deep path not truncated: %q", out)
	}
}

func TestCWDHidesWhenEmpty(t *testing.T) {
	if _, vis := (&CWD{}).Render(&Context{}); vis {
		t.Errorf("expected hidden")
	}
}
