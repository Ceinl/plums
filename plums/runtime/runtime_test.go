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

func TestHasNoConfigArg(t *testing.T) {
	for _, args := range [][]string{
		{"--no-config"},
		{"-no-config"},
		{"--no-config=true"},
		{"-no-config=1"},
	} {
		if !hasNoConfigArg(args) {
			t.Fatalf("hasNoConfigArg(%v) = false, want true", args)
		}
	}
	for _, args := range [][]string{
		{},
		{"--doctor"},
		{"--no-config=false"},
		{"--", "--no-config"},
	} {
		if hasNoConfigArg(args) {
			t.Fatalf("hasNoConfigArg(%v) = true, want false", args)
		}
	}
}

func TestConfigBuildSourceUsesReleasedVersion(t *testing.T) {
	version, dir := configBuildSource("v1.2.3")
	if version != "v1.2.3" {
		t.Fatalf("version = %q", version)
	}
	if dir != "" {
		t.Fatalf("dir = %q, want empty for released version", dir)
	}
}
