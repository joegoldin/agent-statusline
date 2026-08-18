package render

import "testing"

func TestIntentSGRMatchesLegacyHelpers(t *testing.T) {
	// The whole refactor rests on this: every intent must re-emit exactly the
	// bytes the old helper emitted, or the Claude goldens move.
	cases := []struct {
		intent Intent
		legacy func(string) string
	}{
		{IntentDim, Dim},
		{IntentOK, Green},
		{IntentWarn, Yellow},
		{IntentPath, Yellow},
		{IntentCaution, Orange},
		{IntentDanger, Red},
		{IntentAccent, Cyan},
		{IntentMeta, Magenta},
	}
	for _, c := range cases {
		if got, want := c.intent.Wrap("x"), c.legacy("x"); got != want {
			t.Errorf("%s.Wrap = %q, legacy = %q", c.intent, got, want)
		}
		if got := c.intent.Wrap(""); got != "" {
			t.Errorf("%s.Wrap(empty) = %q, want empty", c.intent, got)
		}
	}
}

func TestIntentTextIsUnwrapped(t *testing.T) {
	if got := IntentText.Wrap(" | "); got != " | " {
		t.Errorf("IntentText.Wrap = %q, want the input verbatim", got)
	}
}

func TestSpansANSIConcatenates(t *testing.T) {
	got := Spans{
		Text(IntentOK, " main"),
		Text(IntentText, " "),
		Text(IntentDim, "up2"),
	}.ANSI()
	want := Green(" main") + " " + Dim("up2")
	if got != want {
		t.Errorf("ANSI() = %q, want %q", got, want)
	}
}

func TestSpansANSIRendersBarThroughGradientBar(t *testing.T) {
	got := Spans{Bar(0.535, 10, BarBraille)}.ANSI()
	want := GradientBar(53.5, 10, BrailleStyle)
	if got != want {
		t.Errorf("bar span ANSI mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestSpansANSIAppliesLinkOutsideColour(t *testing.T) {
	got := Spans{Link(IntentAccent, "https://x/1", "#12 approved")}.ANSI()
	want := Hyperlink("https://x/1", Cyan("#12 approved"))
	if got != want {
		t.Errorf("linked span = %q, want %q", got, want)
	}
}

func TestThresholdIntentsMatchThresholdColors(t *testing.T) {
	for _, pct := range []float64{0, 29.9, 30, 44.9, 45, 59.9, 60, 74.9, 75, 84.9, 85, 100} {
		if got, want := ThresholdIntent(pct).Wrap("x"), ThresholdColor(pct)("x"); got != want {
			t.Errorf("ThresholdIntent(%v) = %q, ThresholdColor = %q", pct, got, want)
		}
		if got, want := ThresholdIntent5(pct).Wrap("x"), ThresholdColor5(pct)("x"); got != want {
			t.Errorf("ThresholdIntent5(%v) = %q, ThresholdColor5 = %q", pct, got, want)
		}
	}
}

func TestUnknownBarStyleFallsBackToBlock(t *testing.T) {
	// A snapshot from a newer binary must never panic an older renderer.
	if got := (Spans{Bar(0.5, 4, "no-such-style")}).ANSI(); got != GradientBar(50, 4, BlockStyle) {
		t.Errorf("unknown style did not fall back to block")
	}
}
