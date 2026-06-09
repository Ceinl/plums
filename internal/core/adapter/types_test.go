package adapter

import (
	"strings"
	"testing"
)

func TestDisplayTextForPart(t *testing.T) {
	cases := []struct {
		name string
		part Part
		want string
	}{
		{"text", Part{Type: "text", Text: "hello"}, "hello"},
		{"reasoning wrapped", Part{Type: "reasoning", Text: "hmm"}, "<thinking>hmm</thinking>"},
		{"thinking wrapped", Part{Type: "thinking", Text: "hmm"}, "<thinking>hmm</thinking>"},
		{"unknown ignored", Part{Type: "tool", Text: "x"}, ""},
		{"empty text", Part{Type: "text", Text: ""}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DisplayTextForPart(tc.part); got != tc.want {
				t.Errorf("DisplayTextForPart(%v) = %q, want %q", tc.part, got, tc.want)
			}
		})
	}
}

func TestDefaultPortForDir(t *testing.T) {
	if got := DefaultPortForDir(""); got != 4096 {
		t.Errorf("empty dir port = %d, want 4096", got)
	}
	a := DefaultPortForDir("/some/project")
	b := DefaultPortForDir("/some/project")
	if a != b {
		t.Errorf("port not deterministic: %d != %d", a, b)
	}
	if a < 1024 || a > 49151 {
		t.Errorf("port %d outside registered range", a)
	}
	if DefaultPortForDir("/some/project") == DefaultPortForDir("/other/project") {
		t.Log("warning: two dirs hashed to same port (possible but unlikely)")
	}
}

func TestDefaultBaseURLForDir(t *testing.T) {
	url := DefaultBaseURLForDir("/some/project")
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("unexpected URL %q", url)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()
	if cfg.DefaultBaseURL != DefaultBaseURL {
		t.Errorf("DefaultBaseURL = %q, want %q", cfg.DefaultBaseURL, DefaultBaseURL)
	}
	if cfg.SpinnerInterval <= 0 || cfg.HealthTimeout <= 0 || cfg.ListTimeout <= 0 {
		t.Error("timeouts must be positive")
	}
	if cfg.DefaultOutputPercent <= 0 || cfg.DefaultOutputPercent > 100 {
		t.Errorf("DefaultOutputPercent = %d, want within (0,100]", cfg.DefaultOutputPercent)
	}
}
