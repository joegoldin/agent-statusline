package emit

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/render"
	"github.com/joegoldin/agent-statusline/internal/widgets"
)

func testCtx() *widgets.Context {
	used := 53.5
	return &widgets.Context{
		Mode: input.ModePi,
		Cfg:  config.Defaults(),
		Now:  time.Unix(1748260800, 0).UTC(),
		Status: input.Status{
			CWD:   "/home/joe/p",
			Model: input.Model{ID: "gpt-5.6-sol", DisplayName: "Sol"},
			ContextWindow: &input.ContextWindow{
				ContextWindowSize: 400_000,
				UsedPercentage:    &used,
				TotalInputTokens:  214_000,
			},
		},
	}
}

func testRegistry() widgets.Registry {
	r := widgets.Registry{}
	for _, w := range []widgets.Widget{widgets.Model{}, widgets.CWD{}, widgets.ContextBar{}, widgets.Tokens{}, widgets.Cost{}} {
		r[w.Name()] = w
	}
	return r
}

func TestBuildEmitsEveryConfiguredWidgetIncludingHiddenOnes(t *testing.T) {
	s := Build(testCtx(), testRegistry(), []string{"cost", "context", "model"})
	for _, name := range []string{"model", "cwd", "context", "tokens", "cost"} {
		if _, ok := s.Widgets[name]; !ok {
			t.Errorf("widget %q missing from the snapshot", name)
		}
	}
	// A hidden widget is present with visible:false, never absent. The renderer
	// must be able to tell "configured but hidden" from "unknown widget".
	if s.Widgets["cost"].Visible {
		t.Error("cost should be hidden with no cost recorded")
	}
}

func TestBuildCarriesNoTerminalWidth(t *testing.T) {
	ctx := testCtx()
	ctx.Width = 200
	wide, _ := json.Marshal(Build(ctx, testRegistry(), nil).Widgets)
	ctx.Width = 30
	narrow, _ := json.Marshal(Build(ctx, testRegistry(), nil).Widgets)
	if !bytes.Equal(wide, narrow) {
		t.Error("snapshot widgets vary with ctx.Width; the emitter must be width-independent")
	}
}

func TestBuildEmitsCompactOnlyWhenItDiffers(t *testing.T) {
	s := Build(testCtx(), testRegistry(), nil)
	if s.Widgets["context"].Compact == nil {
		t.Error("context has a distinct compact form and must emit it")
	}
	if s.Widgets["model"].Compact != nil {
		t.Error("model has no compact form; emitting one bloats the wire and the golden")
	}
}

func TestBuildEmitsFillFractionsWithinRange(t *testing.T) {
	s := Build(testCtx(), testRegistry(), nil)
	for name, w := range s.Widgets {
		all := append(append(render.Spans{}, w.Spans...), w.Compact...)
		for _, sp := range all {
			if sp.Kind != "bar" {
				continue
			}
			if sp.Fill < 0 || sp.Fill > 1 {
				t.Errorf("%s: bar fill %v outside [0,1]", name, sp.Fill)
			}
			if sp.Cells <= 0 {
				t.Errorf("%s: bar cells = %d", name, sp.Cells)
			}
		}
	}
}

func TestBuildEchoesConfigSoTheRendererNeedsNoFile(t *testing.T) {
	ctx := testCtx()
	ctx.Cfg.BarWidth = 8
	ctx.Cfg.RefreshInterval = 2
	ctx.Cfg.Widgets.Hide = []string{"pr"}
	s := Build(ctx, testRegistry(), []string{"cost"})
	if s.Config.BarWidth != 8 {
		t.Errorf("BarWidth = %d, want 8", s.Config.BarWidth)
	}
	if s.Config.RefreshIntervalMs != 2000 {
		t.Errorf("RefreshIntervalMs = %d, want 2000", s.Config.RefreshIntervalMs)
	}
	if len(s.Config.Hide) != 1 || s.Config.Hide[0] != "pr" {
		t.Errorf("Hide = %v", s.Config.Hide)
	}
	if len(s.Config.DropPriority) != 1 || s.Config.DropPriority[0] != "cost" {
		t.Errorf("DropPriority = %v", s.Config.DropPriority)
	}
	if s.Config.CompactWidth != widgets.DefaultCompactWidth {
		t.Errorf("CompactWidth = %d, want %d", s.Config.CompactWidth, widgets.DefaultCompactWidth)
	}
}

func TestWriteIsDeterministic(t *testing.T) {
	s := Build(testCtx(), testRegistry(), nil)
	var a, b bytes.Buffer
	if err := Write(&a, s); err != nil {
		t.Fatal(err)
	}
	if err := Write(&b, s); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("Write is not deterministic; the golden would flap")
	}
	if a.Bytes()[a.Len()-1] != '\n' {
		t.Error("Write must end with a newline")
	}
}

func TestBuildSnapshotsTheExtraRows(t *testing.T) {
	ctx := testCtx()
	ctx.Status.AutoMode = "AM● a:9 d:1"
	reg := testRegistry()
	reg[widgets.AutoMode{}.Name()] = widgets.AutoMode{}
	reg[widgets.Cache{}.Name()] = widgets.Cache{}

	s := Build(ctx, reg, nil)
	if !s.Widgets["autoMode"].Visible {
		t.Error("autoMode configured on row3 but missing from the snapshot")
	}
	// The cache sidecar is absent in this context, so the widget must be
	// carried as configured-but-hidden rather than dropped.
	if snap, ok := s.Widgets["cache"]; !ok || snap.Visible {
		t.Errorf("cache snapshot = %+v, want present and hidden", snap)
	}
	if want := []string{"autoMode", "cache"}; !reflect.DeepEqual(s.Config.Row3, want) {
		t.Errorf("Config.Row3 = %v, want %v", s.Config.Row3, want)
	}
	if len(s.Config.Row4) != 0 {
		t.Errorf("Config.Row4 = %v, want empty", s.Config.Row4)
	}
}
