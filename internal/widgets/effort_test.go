package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestEffortRenders(t *testing.T) {
	out, vis := (&Effort{}).Render(&Context{Status: input.Status{Effort: &input.Effort{Level: "high"}}})
	if !vis || !strings.Contains(out, "high") {
		t.Errorf("got %q", out)
	}
}

func TestEffortHides(t *testing.T) {
	if _, vis := (&Effort{}).Render(&Context{}); vis {
		t.Errorf("expected hidden")
	}
}
