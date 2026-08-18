// Package widgets defines a Widget interface that every dashboard segment
// implements. A widget receives a render Context and returns the text it
// would draw (with ANSI codes) plus a visible flag. The layout package
// composes widgets into rows and handles hide-when-empty collapsing.
package widgets

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
