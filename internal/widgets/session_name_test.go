package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestSessionNameRenders(t *testing.T) {
	out, vis := (&SessionName{}).Render(&Context{Status: input.Status{SessionName: "morning-hack"}})
	if !vis || !strings.Contains(out, "morning-hack") {
		t.Errorf("got %q", out)
	}
}

func TestSessionNameHides(t *testing.T) {
	if _, vis := (&SessionName{}).Render(&Context{}); vis {
		t.Errorf("expected hidden")
	}
}
