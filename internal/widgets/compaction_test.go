package widgets

import (
	"strings"
	"testing"
)

func TestCompactionRenders(t *testing.T) {
	w := &Compaction{}
	ctx := &Context{CompactionProvider: func() int { return 2 }}
	out, vis := w.Render(ctx)
	if !vis || !strings.Contains(out, "2c") {
		t.Errorf("got %q", out)
	}
}

func TestCompactionHidesAtZero(t *testing.T) {
	w := &Compaction{}
	if _, vis := w.Render(&Context{CompactionProvider: func() int { return 0 }}); vis {
		t.Errorf("expected hidden")
	}
	if _, vis := w.Render(&Context{}); vis {
		t.Errorf("expected hidden without provider")
	}
}
