package adapter

import (
	"strings"
	"testing"
)

func TestTitleFromMessage(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"simple", "fix the build", "fix the build"},
		{"first line only", "fix the build\nplease and thanks", "fix the build"},
		{"trims whitespace", "  hello  \nworld", "hello"},
		{"empty falls back", "   ", "fallback"},
		{"long is truncated", strings.Repeat("a", 80), strings.Repeat("a", 60) + "…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TitleFromMessage(tc.text, "fallback"); got != tc.want {
				t.Errorf("TitleFromMessage(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}
