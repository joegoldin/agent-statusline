package render

import "testing"

func TestThresholdColor(t *testing.T) {
	tests := []struct {
		pct  float64
		want func(string) string
	}{
		{0, Green},
		{50, Green},
		{69.9, Green},
		{70, Yellow},
		{80, Yellow},
		{84.9, Yellow},
		{85, Red},
		{100, Red},
	}
	for _, tc := range tests {
		got := ThresholdColor(tc.pct)
		wantSample := tc.want("x")
		gotSample := got("x")
		if gotSample != wantSample {
			t.Errorf("ThresholdColor(%v) returned wrong color: got %q want %q",
				tc.pct, gotSample, wantSample)
		}
	}
}

func TestThresholdColor5(t *testing.T) {
	tests := []struct {
		pct  float64
		want func(string) string
	}{
		{0, Dim},
		{15, Dim},
		{29.9, Dim},
		{30, Green},
		{44.9, Green},
		{45, Yellow},
		{59.9, Yellow},
		{60, Orange},
		{74.9, Orange},
		{75, Red},
		{100, Red},
	}
	for _, tc := range tests {
		got := ThresholdColor5(tc.pct)("x")
		want := tc.want("x")
		if got != want {
			t.Errorf("ThresholdColor5(%v): got %q want %q", tc.pct, got, want)
		}
	}
}
