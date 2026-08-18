package render

// Intent is a semantic colour role. It is what widgets declare and what the
// pi snapshot carries on the wire; the literal colour is chosen by whichever
// encoder consumes it. Go's encoder (Intent.SGR) reproduces exactly the ANSI
// the hand-written helpers used to emit, which is why the Claude golden files
// do not move when a widget is converted. pi's encoder maps the same intents
// onto theme tokens, so the statusline finally follows /theme.
type Intent string

const (
	IntentText    Intent = "text"
	IntentDim     Intent = "dim"
	IntentMuted   Intent = "muted"
	IntentAccent  Intent = "accent"
	IntentMeta    Intent = "meta"
	IntentPath    Intent = "path"
	IntentOK      Intent = "ok"
	IntentWarn    Intent = "warn"
	IntentCaution Intent = "caution"
	IntentDanger  Intent = "danger"
)

// SGR returns the parameter bytes for the intent's foreground colour, or ""
// for IntentText (emitted unwrapped). IntentPath and IntentWarn deliberately
// collide on 33 here — Claude Code's palette has no separate colour for a
// path — while pi's encoder splits them.
func (i Intent) SGR() string {
	switch i {
	case IntentDim, IntentMuted:
		return "2"
	case IntentAccent:
		return "36"
	case IntentMeta:
		return "35"
	case IntentPath, IntentWarn:
		return "33"
	case IntentOK:
		return "32"
	case IntentCaution:
		return "38;5;208"
	case IntentDanger:
		return "31"
	}
	return ""
}

// Wrap colours s. An empty string stays empty (no dangling escapes), matching
// the behaviour of the helpers this replaces.
func (i Intent) Wrap(s string) string {
	sgr := i.SGR()
	if sgr == "" {
		return s
	}
	return wrap(sgr, s)
}

// Bar style names, carried on the wire so the pi renderer can pick its own
// glyph set without the Go side shipping glyphs it will not draw.
const (
	BarBraille = "braille"
	BarBlock   = "block"
	BarLine    = "line"
)

// Span is one atom of a widget's output: either a run of text with a colour
// intent, or a progress bar expressed as a fill fraction. It is the JSON wire
// shape for --emit json; the omitempty tags keep the snapshot readable.
type Span struct {
	Kind   string  `json:"kind"`
	Text   string  `json:"text,omitempty"`
	Intent Intent  `json:"intent,omitempty"`
	Link   string  `json:"link,omitempty"`
	Fill   float64 `json:"fill,omitempty"`
	Cells  int     `json:"cells,omitempty"`
	Style  string  `json:"style,omitempty"`
}

// Spans is a widget's full output, in draw order.
type Spans []Span

// Text builds a coloured text span.
func Text(i Intent, s string) Span { return Span{Kind: "text", Text: s, Intent: i} }

// Link builds a coloured text span wrapped in an OSC 8 hyperlink.
func Link(i Intent, url, s string) Span {
	return Span{Kind: "text", Text: s, Intent: i, Link: url}
}

// Bar builds a progress-bar span. fill is a fraction in [0,1]; the pi renderer
// needs the fraction rather than a percentage because it re-derives per-cell
// colours from the active theme's ramp.
func Bar(fill float64, cells int, style string) Span {
	return Span{Kind: "bar", Fill: fill, Cells: cells, Style: style}
}

func barStyle(name string) BarStyle {
	switch name {
	case BarBraille:
		return BrailleStyle
	case BarLine:
		return LineStyle
	}
	return BlockStyle
}

// ANSI encodes spans for a terminal that has no theme of its own — i.e. the
// Claude Code path, and --emit ansi generally.
func (s Spans) ANSI() string {
	out := ""
	for _, sp := range s {
		switch sp.Kind {
		case "bar":
			out += GradientBar(sp.Fill*100, sp.Cells, barStyle(sp.Style))
		default:
			t := sp.Intent.Wrap(sp.Text)
			if sp.Link != "" && t != "" {
				t = Hyperlink(sp.Link, t)
			}
			out += t
		}
	}
	return out
}
