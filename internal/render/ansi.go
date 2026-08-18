// Package render emits ANSI 16-color foreground SGR codes and OSC 8 hyperlinks.
// The terminal's theme handles RGB; this package only emits semantic colors.
package render

const reset = "\x1b[0m"

func wrap(code, s string) string {
	if s == "" {
		return ""
	}
	return "\x1b[" + code + "m" + s + reset
}

// Semantic colors (ANSI 16), kept as functions for the many call sites that
// predate intents. Each is now a thin alias over the intent table in span.go,
// so the two can never drift. Orange uses ANSI 256-color index 208 (xterm
// orange): there is no orange in the ANSI 16 palette but we want a clear step
// between yellow and red for the five-level threshold gradient.
func Dim(s string) string     { return IntentDim.Wrap(s) }
func Red(s string) string     { return IntentDanger.Wrap(s) }
func Green(s string) string   { return IntentOK.Wrap(s) }
func Yellow(s string) string  { return IntentWarn.Wrap(s) }
func Magenta(s string) string { return IntentMeta.Wrap(s) }
func Cyan(s string) string    { return IntentAccent.Wrap(s) }
func Orange(s string) string  { return IntentCaution.Wrap(s) }

// Hyperlink wraps text in an OSC 8 hyperlink. Callers should pass already-
// rendered (colored) text; this only adds the link layer.
func Hyperlink(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
