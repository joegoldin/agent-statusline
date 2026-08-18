package render

import "testing"

func TestColor(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"cyan", Cyan("hi"), "\x1b[36mhi\x1b[0m"},
		{"dim", Dim("hi"), "\x1b[2mhi\x1b[0m"},
		{"green", Green("ok"), "\x1b[32mok\x1b[0m"},
		{"yellow", Yellow("warn"), "\x1b[33mwarn\x1b[0m"},
		{"red", Red("bad"), "\x1b[31mbad\x1b[0m"},
		{"magenta", Magenta("hl"), "\x1b[35mhl\x1b[0m"},
		{"yellow + empty stays empty", Yellow(""), ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q want %q", tc.got, tc.want)
			}
		})
	}
}

func TestHyperlink(t *testing.T) {
	got := Hyperlink("https://example.com", "click me")
	want := "\x1b]8;;https://example.com\x1b\\click me\x1b]8;;\x1b\\"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
