package app

import (
	"strings"
	"testing"
)

func TestFormatVersionMetadata(t *testing.T) {
	got := FormatVersionMetadata(VersionMetadata{
		Version:   "1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-05-29T00:00:00Z",
		GoVersion: "go1.26.2",
		Modified:  true,
	})
	want := "plums 1.2.3\ncommit: abc123\nbuildDate: 2026-05-29T00:00:00Z\ngo: go1.26.2\nmodified: true\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestLoadVersionMetadataDefaults(t *testing.T) {
	got := LoadVersionMetadata("0.1.0-dev", "unknown", "unknown")
	if got.Version != "0.1.0-dev" {
		t.Fatalf("expected app version %q, got %q", "0.1.0-dev", got.Version)
	}
	if got.Commit == "" {
		t.Fatal("expected commit")
	}
	if got.BuildDate == "" {
		t.Fatal("expected build date")
	}
	if !strings.HasPrefix(got.GoVersion, "go") {
		t.Fatalf("expected go version, got %q", got.GoVersion)
	}
}
