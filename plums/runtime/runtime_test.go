package runtime

import "testing"

func TestDefaultConfigDelegatesToRuntimeConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.BackendProvider == "" {
		t.Fatal("BackendProvider must have a default")
	}
}

func TestIsBuildCommand(t *testing.T) {
	for _, arg := range []string{"build", "--recompile", "-recompile"} {
		if !isBuildCommand(arg) {
			t.Fatalf("isBuildCommand(%q) = false, want true", arg)
		}
	}
	if isBuildCommand("doctor") {
		t.Fatal("doctor should not dispatch to build")
	}
}
