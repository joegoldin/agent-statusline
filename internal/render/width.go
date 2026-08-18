package render

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/width"
)

// VisibleWidth returns the on-screen cell width of s, ignoring ANSI/OSC
// escape sequences. CJK ideographs and most emoji count as 2 cells.
func VisibleWidth(s string) int {
	total := 0
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == 0x1b && i+1 < len(s) && s[i+1] == '[': // CSI sequence
			i = skipCSI(s, i)
		case r == 0x1b && i+1 < len(s) && s[i+1] == ']': // OSC sequence
			i = skipOSC(s, i)
		default:
			total += runeWidth(r)
			i += n
		}
	}
	return total
}

func runeWidth(r rune) int {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianFullwidth, width.EastAsianWide:
		return 2
	}
	// Common emoji ranges (rough heuristic; covers most pictographs).
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF:
		return 2
	case r >= 0x2600 && r <= 0x27BF:
		return 2
	}
	return 1
}

func skipCSI(s string, i int) int {
	// ESC [ ... <final byte 0x40-0x7e>
	j := i + 2
	for j < len(s) {
		c := s[j]
		j++
		if c >= 0x40 && c <= 0x7e {
			return j
		}
	}
	return len(s)
}

func skipOSC(s string, i int) int {
	// ESC ] ... terminated by BEL (0x07) or ESC \
	j := i + 2
	for j < len(s) {
		c := s[j]
		if c == 0x07 {
			return j + 1
		}
		if c == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
			return j + 2
		}
		j++
	}
	return len(s)
}

// Truncate clamps s to maxWidth visible cells, appending "…" if anything was
// dropped. Any open OSC 8 hyperlink is closed at the end so terminals don't
// keep the link "stuck" past the truncation.
func Truncate(s string, maxWidth int) string {
	if VisibleWidth(s) <= maxWidth {
		return s
	}
	if maxWidth < 1 {
		return ""
	}
	limit := maxWidth - 1 // reserve a cell for the ellipsis
	var b strings.Builder
	used := 0
	inLink := false
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			end := skipCSI(s, i)
			b.WriteString(s[i:end])
			i = end
			continue
		}
		if r == 0x1b && i+1 < len(s) && s[i+1] == ']' {
			end := skipOSC(s, i)
			seq := s[i:end]
			inLink = isOpenHyperlink(seq)
			b.WriteString(seq)
			i = end
			continue
		}
		w := runeWidth(r)
		if used+w > limit {
			break
		}
		used += w
		b.WriteString(s[i : i+n])
		i += n
	}
	b.WriteString("…")
	if inLink {
		b.WriteString("\x1b]8;;\x1b\\")
	}
	return b.String()
}

// TruncateMiddle clamps s to maxWidth visible cells by dropping the middle and
// inserting "…", so both the start and the end stay visible (e.g. a long shell
// command shows its leading subcommand and trailing args). It assumes s holds
// no ANSI escapes — callers truncate raw text before colorizing.
func TruncateMiddle(s string, maxWidth int) string {
	if maxWidth < 1 {
		return ""
	}
	if VisibleWidth(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}
	avail := maxWidth - 1 // reserve a cell for the ellipsis
	headW := (avail + 1) / 2
	tailW := avail - headW

	runes := []rune(s)
	var head strings.Builder
	used := 0
	hi := 0
	for ; hi < len(runes); hi++ {
		w := runeWidth(runes[hi])
		if used+w > headW {
			break
		}
		head.WriteRune(runes[hi])
		used += w
	}
	used = 0
	ti := len(runes)
	for ti > hi {
		w := runeWidth(runes[ti-1])
		if used+w > tailW {
			break
		}
		used += w
		ti--
	}
	return head.String() + "…" + string(runes[ti:])
}

// isOpenHyperlink reports whether an OSC 8 sequence opens a link
// (i.e. has a non-empty URL between the two `;;` markers).
func isOpenHyperlink(seq string) bool {
	const prefix = "\x1b]8;;"
	if !strings.HasPrefix(seq, prefix) {
		return false
	}
	body := strings.TrimPrefix(seq, prefix)
	for i := 0; i < len(body); i++ {
		if body[i] == 0x07 || (body[i] == 0x1b && i+1 < len(body) && body[i+1] == '\\') {
			return i > 0
		}
	}
	return false
}

// StripANSI returns s with all CSI and OSC escape sequences removed.
// Useful in tests for asserting on visible content alone.
func StripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if i+1 < len(s) && s[i] == 0x1b && s[i+1] == '[' {
			i = skipCSI(s, i)
			continue
		}
		if i+1 < len(s) && s[i] == 0x1b && s[i+1] == ']' {
			i = skipOSC(s, i)
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
