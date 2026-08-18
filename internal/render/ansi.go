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

// Semantic colors (ANSI 16).
func Dim(s string) string     { return wrap("2", s) }
func Red(s string) string     { return wrap("31", s) }
func Green(s string) string   { return wrap("32", s) }
func Yellow(s string) string  { return wrap("33", s) }
func Magenta(s string) string { return wrap("35", s) }
func Cyan(s string) string    { return wrap("36", s) }

// Orange uses ANSI 256-color index 208 (xterm orange). There is no
// orange in the ANSI 16 palette but we want a clear step between yellow
// and red for the five-level threshold gradient. All modern terminals
// support 256-color SGR; older terminals degrade to the closest match.
func Orange(s string) string { return wrap("38;5;208", s) }

// Hyperlink wraps text in an OSC 8 hyperlink. Callers should pass already-
// rendered (colored) text; this only adds the link layer.
func Hyperlink(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
