package layout

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/render"
	"github.com/joegoldin/agent-statusline/internal/widgets"
)

type fake struct {
	name string
	out  string
	vis  bool
}

func (f fake) Name() string                           { return f.name }
func (f fake) Render(*widgets.Context) (string, bool) { return f.out, f.vis }

func TestWrapRowPacksAcrossLines(t *testing.T) {
	// Three ~10-wide segments; width 24 fits two per line (10 + 3 sep + 10 = 23).
	row := []widgets.Widget{
		fake{"a", "AAAAAAAAAA", true},
		fake{"b", "BBBBBBBBBB", true},
		fake{"c", "CCCCCCCCCC", true},
	}
	lines := WrapRow(row, &widgets.Context{}, Options{Width: 24})
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "AAAAAAAAAA") || !strings.Contains(lines[0], "BBBBBBBBBB") {
		t.Errorf("line 0 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "CCCCCCCCCC") {
		t.Errorf("line 1 = %q", lines[1])
	}
}

func TestWrapRowSkipsHiddenAndFlex(t *testing.T) {
	row := []widgets.Widget{
		fake{"a", "A", true},
		FlexMarker(),
		fake{"b", "", false}, // hidden (not visible)
		fake{"c", "C", true},
	}
	lines := WrapRow(row, &widgets.Context{}, Options{Width: 80})
	if len(lines) != 1 || !strings.Contains(lines[0], "A") || !strings.Contains(lines[0], "C") {
		t.Errorf("got %q", lines)
	}
	if strings.Contains(lines[0], "  ") {
		t.Errorf("flex should be ignored in wrap mode: %q", lines[0])
	}
}

func TestComposeRowSimple(t *testing.T) {
	row := []widgets.Widget{
		fake{"model", "Opus", true},
		fake{"cwd", "claude-nix", true},
	}
	out := ComposeRow(row, nil, &widgets.Context{}, Options{Width: 80, DropPriority: nil})
	if !strings.Contains(out, "Opus") || !strings.Contains(out, "claude-nix") {
		t.Errorf("got %q", out)
	}
	if !strings.Contains(out, " │ ") {
		t.Errorf("expected separator in %q", out)
	}
}

func TestComposeRowSeparatorCollapsesAroundHidden(t *testing.T) {
	row := []widgets.Widget{
		fake{"a", "A", true},
		fake{"b", "B", false},
		fake{"c", "C", true},
	}
	out := ComposeRow(row, nil, &widgets.Context{}, Options{Width: 80})
	if strings.Contains(out, "│ │") {
		t.Errorf("dangling separator in %q", out)
	}
	if !strings.Contains(out, "A") || !strings.Contains(out, "C") {
		t.Errorf("got %q", out)
	}
}

func TestComposeRowFlexSpacerRightAligns(t *testing.T) {
	row := []widgets.Widget{
		fake{"a", "A", true},
		flexMarker{},
		fake{"b", "B", true},
	}
	out := ComposeRow(row, nil, &widgets.Context{}, Options{Width: 20})
	if !strings.HasPrefix(render.StripANSI(out), "A") {
		t.Errorf("not left-anchored: %q", out)
	}
	if !strings.HasSuffix(render.StripANSI(out), "B") {
		t.Errorf("not right-anchored: %q", out)
	}
}

func TestComposeRowOverflowDropsByPriority(t *testing.T) {
	row := []widgets.Widget{
		fake{"low", strings.Repeat("L", 10), true},
		fake{"high", strings.Repeat("H", 10), true},
	}
	priority := []string{"low", "high"}
	out := ComposeRow(row, nil, &widgets.Context{}, Options{Width: 12, DropPriority: priority})
	if strings.Contains(out, "LLLLLLLLLL") {
		t.Errorf("low-priority segment should have dropped: %q", out)
	}
	if !strings.Contains(out, "HHHHHHHHHH") {
		t.Errorf("high-priority segment missing: %q", out)
	}
}
