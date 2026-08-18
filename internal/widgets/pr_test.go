package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestPRRenders(t *testing.T) {
	w := &PR{}
	out, vis := w.Render(&Context{Status: input.Status{PR: &input.PR{
		Number: 42, URL: "https://example.com/pr/42", ReviewState: "approved",
	}}})
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "42") || !strings.Contains(out, "approved") {
		t.Errorf("got %q", out)
	}
	if !strings.Contains(out, "\x1b]8;;https://example.com/pr/42\x1b\\") {
		t.Errorf("expected OSC 8 hyperlink in %q", out)
	}
}

func TestPRHides(t *testing.T) {
	if _, vis := (&PR{}).Render(&Context{}); vis {
		t.Errorf("expected hidden")
	}
}
