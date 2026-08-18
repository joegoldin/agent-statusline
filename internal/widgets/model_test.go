package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestModelWidgetBasic(t *testing.T) {
	w := &Model{}
	ctx := &Context{Status: input.Status{Model: input.Model{DisplayName: "Opus"}}}
	out, vis := w.Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "Opus") {
		t.Errorf("output missing Opus: %q", out)
	}
}

func TestModelWidgetStripsContextSuffix(t *testing.T) {
	w := &Model{}
	// No ContextWindow on this case, so the 1M-detection shouldn't fire
	// either — verifying we strip the parenthetical AND don't re-add 1M.
	ctx := &Context{Status: input.Status{Model: input.Model{DisplayName: "Opus (1M context)"}}}
	out, _ := w.Render(ctx)
	if strings.Contains(out, "1M") {
		t.Errorf("expected suffix stripped: %q", out)
	}
}

func TestModelWidgetAppends1MFromContextWindowSize(t *testing.T) {
	w := &Model{}
	ctx := &Context{Status: input.Status{
		Model:         input.Model{DisplayName: "Opus 4.7", ID: "claude-opus-4-7"},
		ContextWindow: &input.ContextWindow{ContextWindowSize: 1_000_000},
	}}
	out, _ := w.Render(ctx)
	if !strings.Contains(out, "1M") {
		t.Errorf("expected 1M appended when context_window_size=1M: %q", out)
	}
}

func TestModelWidgetNo1MFor200kContext(t *testing.T) {
	w := &Model{}
	ctx := &Context{Status: input.Status{
		Model:         input.Model{DisplayName: "Opus 4.7", ID: "claude-opus-4-7"},
		ContextWindow: &input.ContextWindow{ContextWindowSize: 200_000},
	}}
	out, _ := w.Render(ctx)
	if strings.Contains(out, "1M") {
		t.Errorf("did not expect 1M for 200k context: %q", out)
	}
}

func TestModelWidgetHidesWhenAbsent(t *testing.T) {
	w := &Model{}
	if _, vis := w.Render(&Context{}); vis {
		t.Errorf("expected hidden when display_name empty")
	}
}
