package app

import (
	"os"
	"path/filepath"
	"testing"

	cfgpkg "github.com/Ceinl/plums/config"
)

func TestStatePrefsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.toml")
	in := map[string]string{
		prefBackend:       "codex",
		prefDefaultLayout: "zen",
		prefHideThinking:  "false",
		prefOutputPercent: "60",
	}
	if err := writeStatePrefs(path, in); err != nil {
		t.Fatalf("write prefs: %v", err)
	}
	got, err := loadStatePrefs(path)
	if err != nil {
		t.Fatalf("load prefs: %v", err)
	}
	for key, want := range in {
		if got[key] != want {
			t.Fatalf("pref %q = %q, want %q", key, got[key], want)
		}
	}
}

func TestLoadStatePrefsMissingFileIsEmpty(t *testing.T) {
	got, err := loadStatePrefs(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatalf("load missing prefs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestApplyDynamicPrefsSeedsDeclaredFields(t *testing.T) {
	prefs := map[string]string{
		prefBackend:       "codex",
		prefDefaultLayout: "zen",
		prefHideThinking:  "false",
		prefOutputPercent: "60",
	}
	opts := cfgpkg.Opts{
		Backend:       cfgpkg.Dynamic,
		DefaultLayout: cfgpkg.Pref("split"), // pinned: must NOT be overwritten
		OutputPercent: cfgpkg.DynamicCount,
		// HideThinking left Unset so the remembered value seeds it.
	}
	got := applyDynamicPrefs(opts, prefs)

	if got.Backend.Value() != "codex" {
		t.Fatalf("Backend = %q, want codex (Dynamic seeded)", got.Backend.Value())
	}
	if got.DefaultLayout.Value() != "split" {
		t.Fatalf("DefaultLayout = %q, want split (pinned, not seeded)", got.DefaultLayout.Value())
	}
	if got.OutputPercent.Value(0) != 60 {
		t.Fatalf("OutputPercent = %d, want 60 (Dynamic seeded)", got.OutputPercent.Value(0))
	}
	if got.HideThinking.Value(true) != false {
		t.Fatal("HideThinking = true, want false (unset seeded from prefs)")
	}
}

func TestApplyDynamicPrefsLeavesDynamicWhenAbsent(t *testing.T) {
	opts := cfgpkg.Opts{Backend: cfgpkg.Dynamic}
	got := applyDynamicPrefs(opts, map[string]string{})
	if !got.Backend.IsDynamic() {
		t.Fatal("expected Backend to remain Dynamic so the default fallback wins")
	}
}

func TestDynamicPrefWriterPersistsOnlyDeclared(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	w := NewDynamicPrefWriter(cfgpkg.Opts{
		Backend:       cfgpkg.Dynamic,       // declared -> persisted
		DefaultLayout: cfgpkg.Pref("split"), // pinned -> dropped
	})
	if err := w.Persist(map[string]string{
		prefBackend:       "codex",
		prefDefaultLayout: "zen",
	}); err != nil {
		t.Fatalf("persist: %v", err)
	}
	path := filepath.Join(dir, ".config", "plums", "state.toml")
	prefs, err := loadStatePrefs(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if prefs[prefBackend] != "codex" {
		t.Fatalf("backend = %q, want codex", prefs[prefBackend])
	}
	if _, ok := prefs[prefDefaultLayout]; ok {
		t.Fatal("default_layout should not persist (pinned, not Dynamic)")
	}
}

func TestApplyDynamicPrefsRoundTripFromDisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "plums", "state.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writeStatePrefs(path, map[string]string{prefBackend: "claude"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := ApplyDynamicPrefs(cfgpkg.Opts{Backend: cfgpkg.Dynamic})
	if got.Backend.Value() != "claude" {
		t.Fatalf("Backend = %q, want claude (seeded from disk)", got.Backend.Value())
	}
}
