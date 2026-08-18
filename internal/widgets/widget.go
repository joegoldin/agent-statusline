// Package widgets defines a Widget interface that every dashboard segment
// implements. A widget receives a render Context and returns the text it
// would draw (with ANSI codes) plus a visible flag. The layout package
// composes widgets into rows and handles hide-when-empty collapsing.
package widgets

import "github.com/joegoldin/agent-statusline/internal/render"

// Widget is the contract every dashboard segment implements.
type Widget interface {
	// Name is the lowercase identifier used in config (e.g. "model", "cwd").
	Name() string
	// Render returns (text, visible). When visible is false, the layout
	// drops the widget and collapses the surrounding separators.
	Render(ctx *Context) (string, bool)
}

// Registry maps widget names to their implementations. The main entry point
// builds the registry once with all dependencies wired in.
type Registry map[string]Widget

// Lookup returns the widget for name, or nil if unknown.
func (r Registry) Lookup(name string) Widget { return r[name] }

// SafeRender wraps Render so any panic is converted into a hidden widget.
func SafeRender(w Widget, ctx *Context) (text string, visible bool) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
			visible = false
		}
	}()
	return w.Render(ctx)
}

// SpanRenderer is the semantic half of a widget: the same content Render
// produces, but as intents rather than escape codes. Every widget implements
// it; the interface stays optional so a future widget can be added ANSI-first
// and converted later without breaking the emitter.
type SpanRenderer interface {
	RenderSpans(ctx *Context) (render.Spans, bool)
}

// SafeRenderSpans is SafeRender's span twin. A widget that does not implement
// SpanRenderer degrades to a single text-intent span holding its raw output:
// pi then renders it in the theme's default foreground, losing colour but
// never losing the widget.
func SafeRenderSpans(w Widget, ctx *Context) (spans render.Spans, visible bool) {
	defer func() {
		if r := recover(); r != nil {
			spans, visible = nil, false
		}
	}()
	if sr, ok := w.(SpanRenderer); ok {
		return sr.RenderSpans(ctx)
	}
	text, vis := w.Render(ctx)
	if !vis || text == "" {
		return nil, false
	}
	return render.Spans{render.Text(render.IntentText, text)}, true
}
