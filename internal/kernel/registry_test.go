package kernel

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

func TestEntriesInRegistrationOrder(t *testing.T) {
	reg := NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryBackend, "opencode", 1, "core"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(capabilities.RegistryBackend, "codex", 2, "core"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(capabilities.RegistryBackend, "opencode", 3, "user"); err != nil {
		t.Fatal(err)
	}

	entries := reg.EntriesInRegistrationOrder(capabilities.RegistryBackend)
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if entries[0].Name != "opencode" || entries[0].Owner != "user" || entries[0].Value.(int) != 3 {
		t.Fatalf("first entry = %+v", entries[0])
	}
	if entries[1].Name != "codex" || entries[1].Owner != "core" || entries[1].Value.(int) != 2 {
		t.Fatalf("second entry = %+v", entries[1])
	}
}

func TestEntriesInRegistrationOrderSkipsDisabled(t *testing.T) {
	reg := NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryBackend, "opencode", 1, "core"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(capabilities.RegistryBackend, "codex", 2, "core"); err != nil {
		t.Fatal(err)
	}
	reg.Disable([]capabilities.RegistryKey{{Kind: capabilities.RegistryBackend, Name: "opencode"}})

	entries := reg.EntriesInRegistrationOrder(capabilities.RegistryBackend)
	if len(entries) != 1 || entries[0].Name != "codex" {
		t.Fatalf("entries = %+v", entries)
	}
}
