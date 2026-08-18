package render

import (
	"encoding/json"
	"os"
	"testing"
)

// The corpus is shared with extension/src/width.test.ts, which pins the
// TypeScript port against the real pi-tui. Pinning Go against the same strings
// closes the triangle: Go, our TS port and pi-tui all agree on which strings
// are worth exercising, so a glyph that breaks one is exercised against the
// other. The assertions here are self-consistency rather than equality with
// pi-tui — Go's tab and emoji handling is its own, and the Claude goldens pin
// that half.
func TestVisibleWidthMatchesTheSharedCorpus(t *testing.T) {
	raw, err := os.ReadFile("../../extension/testdata/width-corpus.json")
	if err != nil {
		t.Skipf("corpus unavailable: %v", err)
	}
	var corpus []string
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("corpus is not valid JSON: %v", err)
	}
	if len(corpus) < 20 {
		t.Fatalf("corpus has %d entries; it should cover every glyph class", len(corpus))
	}
	for _, s := range corpus {
		if w := VisibleWidth(s); w < 0 {
			t.Errorf("VisibleWidth(%q) = %d", s, w)
		}
		for _, max := range []int{1, 5, 10, 40} {
			if got := VisibleWidth(Truncate(s, max)); got > max {
				t.Errorf("Truncate(%q, %d) is %d cells wide", s, max, got)
			}
			if got := VisibleWidth(TruncateMiddle(s, max)); got > max {
				t.Errorf("TruncateMiddle(%q, %d) is %d cells wide", s, max, got)
			}
		}
	}
}
