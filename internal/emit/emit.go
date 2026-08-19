// Package emit serialises a rendered statusline into structured JSON, for
// renderers that own their own layout, colours and clock.
//
// The Claude Code path does not use this: it takes the ANSI encoding, where
// the terminal has no theme to consult and the harness re-invokes the binary
// on a timer. pi is the opposite on both counts — it has a live theme proxy
// and it pushes the viewport width into render(width) on every frame — so it
// gets the semantic form and does the drawing itself.
package emit

import (
	"encoding/json"
	"io"
	"reflect"

	"github.com/joegoldin/agent-statusline/internal/render"
	"github.com/joegoldin/agent-statusline/internal/widgets"
)

// SchemaVersion is bumped only for breaking changes. The renderer refuses a
// snapshot whose schema it does not know rather than drawing a wrong line.
const SchemaVersion = 1

// MaxLines mirrors the total line budget main.go applies to the ANSI path, so
// both renderers cap in the same place.
const MaxLines = 6

// Separator and FlexName are inlined rather than imported from internal/layout,
// which already imports internal/widgets and would cycle. A test in
// internal/layout pins the two literals.
const (
	separatorLiteral = " │ "
	flexNameLiteral  = "flex"
)

type Snapshot struct {
	Schema   int                       `json:"schema"`
	Mode     string                    `json:"mode"`
	AsOfMs   int64                     `json:"asOfMs"`
	Config   SnapshotConfig            `json:"config"`
	Widgets  map[string]WidgetSnapshot `json:"widgets"`
	Activity widgets.ActivitySnapshot  `json:"activity"`
}

// SnapshotConfig is the binary's effective configuration, echoed so the
// renderer never reads a config file. One reader, one resolution order, one
// set of defaults — the alternative is two implementations of
// config.ResolvePath drifting apart.
type SnapshotConfig struct {
	BarWidth          int      `json:"barWidth"`
	CompactWidth      int      `json:"compactWidth"`
	ActivityRows      int      `json:"activityRows"`
	HideWhenIdle      bool     `json:"hideWhenIdle"`
	Padding           int      `json:"padding"`
	RefreshIntervalMs int      `json:"refreshIntervalMs"`
	MaxLines          int      `json:"maxLines"`
	Separator         string   `json:"separator"`
	FlexName          string   `json:"flexName"`
	Row1              []string `json:"row1"`
	Row2              []string `json:"row2"`
	Row3              []string `json:"row3"`
	Row4              []string `json:"row4"`
	Hide              []string `json:"hide"`
	DropPriority      []string `json:"dropPriority"`
}

// WidgetSnapshot is one widget. Compact is nil when the widget renders the
// same either way, which keeps the wire and the golden files small.
type WidgetSnapshot struct {
	Visible bool         `json:"visible"`
	Spans   render.Spans `json:"spans,omitempty"`
	Compact render.Spans `json:"compact,omitempty"`
}

// Build renders every configured widget twice — once at full width and once
// narrow enough to trip Compact() — and packages the result. It mutates and
// restores ctx.Width, the only width input the dashboard widgets have, so no
// widget needs a second code path.
func Build(ctx *widgets.Context, reg widgets.Registry, dropPriority []string) Snapshot {
	cfg := ctx.Cfg
	compactWidth := ctx.CompactWidth
	if compactWidth <= 0 {
		compactWidth = widgets.DefaultCompactWidth
	}

	restore := ctx.Width
	defer func() { ctx.Width = restore }()

	names := map[string]bool{}
	for _, row := range [][]string{cfg.Widgets.Row1, cfg.Widgets.Row2, cfg.Widgets.Row3, cfg.Widgets.Row4} {
		for _, n := range row {
			names[n] = true
		}
	}

	out := map[string]WidgetSnapshot{}
	for name := range names {
		w := reg.Lookup(name)
		if w == nil {
			continue // "flex" and any unknown name; the renderer handles flex itself
		}
		ctx.Width = 0 // Compact() is false when width is unknown
		full, visible := widgets.SafeRenderSpans(w, ctx)
		snap := WidgetSnapshot{Visible: visible}
		if !visible {
			out[name] = snap
			continue
		}
		snap.Spans = full
		ctx.Width = compactWidth - 1
		compact, compactVisible := widgets.SafeRenderSpans(w, ctx)
		if compactVisible && !reflect.DeepEqual(compact, full) {
			snap.Compact = compact
		}
		out[name] = snap
	}

	ctx.Width = 0
	refresh := cfg.RefreshInterval
	if refresh <= 0 {
		refresh = 1
	}
	return Snapshot{
		Schema: SchemaVersion,
		Mode:   string(ctx.Mode),
		AsOfMs: ctx.Now.UnixMilli(),
		Config: SnapshotConfig{
			BarWidth:          cfg.BarWidth,
			CompactWidth:      compactWidth,
			ActivityRows:      cfg.ActivityRows,
			HideWhenIdle:      cfg.HideWhenIdle,
			Padding:           cfg.Padding,
			RefreshIntervalMs: refresh * 1000,
			MaxLines:          MaxLines,
			Separator:         separatorLiteral,
			FlexName:          flexNameLiteral,
			Row1:              nonNil(cfg.Widgets.Row1),
			Row2:              nonNil(cfg.Widgets.Row2),
			Row3:              nonNil(cfg.Widgets.Row3),
			Row4:              nonNil(cfg.Widgets.Row4),
			Hide:              nonNil(cfg.Widgets.Hide),
			DropPriority:      nonNil(dropPriority),
		},
		Widgets:  out,
		Activity: widgets.BuildActivitySnapshot(ctx),
	}
}

// nonNil keeps JSON arrays as [] rather than null, so the renderer can iterate
// without a guard on every list.
func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

// Write emits the snapshot indented and newline-terminated. Indented because
// the golden files are reviewed by humans, and a one-line diff on a 6 kB blob
// is not a review.
func Write(w io.Writer, s Snapshot) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(s)
}
