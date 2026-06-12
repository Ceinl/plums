package api

import (
	"flag"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BackendProvider != "opencode" {
		t.Errorf("BackendProvider = %q, want opencode", cfg.BackendProvider)
	}
	if cfg.OpencodeServerURL == "" {
		t.Error("OpencodeServerURL must have a default")
	}
	if cfg.ClipboardCommand == "" {
		t.Error("ClipboardCommand must have a default")
	}
	if !cfg.UseGlobalConfig {
		t.Error("UseGlobalConfig must default to true")
	}
	if cfg.ShowVersion || cfg.InitConfig || cfg.InitLocalConfig {
		t.Error("action flags must default to false")
	}
}

func TestRegisterFlags(t *testing.T) {
	cfg := DefaultConfig()
	RegisterFlags(cfg)
	for _, name := range []string{"server-url", "provider", "config-global", "cg", "config-local", "cl", "init-config", "init-config-local", "version"} {
		if flag.Lookup(name) == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
}

func TestRunShowVersion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShowVersion = true
	cfg.Version = "1.2.3-test"
	if err := Run(cfg); err != nil {
		t.Errorf("Run with ShowVersion returned error: %v", err)
	}
}
