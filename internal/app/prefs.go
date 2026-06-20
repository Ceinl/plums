package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	cfgpkg "github.com/Ceinl/plums/config"
)

// prefsKey identifies a dynamic preference in the state.toml store. The keys are
// stable on-disk names, independent of the Go field names.
const (
	prefBackend       = "backend"
	prefModel         = "model"
	prefDefaultLayout = "default_layout"
	prefMode          = "mode"
	prefHideThinking  = "hide_thinking"
	prefOutputPercent = "output_percent"
	prefSplitLeft     = "split_left_width"
)

// statePrefsPath returns ~/.config/plums/state.toml, the app-managed
// dynamic-preferences store. It is NOT a user-authored config file: it only
// remembers the runtime value of Opts fields the config declared cfg.Dynamic.
func statePrefsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "plums", "state.toml"), nil
}

// loadStatePrefs reads the state.toml store into a flat key→value map. A missing
// file yields an empty map (not an error). The format is a minimal subset of
// TOML: top-level `key = value`, where value is a quoted string, bare integer,
// or bare bool. Comments (#) and blank lines are ignored.
func loadStatePrefs(path string) (map[string]string, error) {
	out := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		out[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// writeStatePrefs persists the prefs map to path, creating parent directories as
// needed. String values are quoted; bare true/false and integers are written
// unquoted so they round-trip cleanly.
func writeStatePrefs(path string, prefs map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	keys := []string{prefBackend, prefModel, prefDefaultLayout, prefMode, prefHideThinking, prefOutputPercent, prefSplitLeft}
	var b strings.Builder
	b.WriteString("# plums dynamic preferences — managed by the app, do not edit by hand.\n")
	for _, key := range keys {
		value, ok := prefs[key]
		if !ok {
			continue
		}
		b.WriteString(formatPrefLine(key, value))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func formatPrefLine(key, value string) string {
	if value == "true" || value == "false" {
		return fmt.Sprintf("%s = %s\n", key, value)
	}
	if _, err := strconv.Atoi(value); err == nil {
		return fmt.Sprintf("%s = %s\n", key, value)
	}
	return fmt.Sprintf("%s = %q\n", key, value)
}

// ApplyDynamicPrefs resolves every Opts field declared cfg.Dynamic to its
// remembered value from the state.toml store. Fields that are not Dynamic are
// left untouched (a pinned literal or unset stays as-is). When the store has no
// value for a Dynamic field, the field is left Dynamic so ToSettings falls back
// to the Default Config value. Only Dynamic fields are ever touched.
func ApplyDynamicPrefs(opts cfgpkg.Opts) cfgpkg.Opts {
	path, err := statePrefsPath()
	if err != nil {
		return opts
	}
	prefs, err := loadStatePrefs(path)
	if err != nil {
		return opts
	}
	return applyDynamicPrefs(opts, prefs)
}

func applyDynamicPrefs(opts cfgpkg.Opts, prefs map[string]string) cfgpkg.Opts {
	if opts.Backend.IsDynamic() {
		if v, ok := prefs[prefBackend]; ok && v != "" {
			opts.Backend = cfgpkg.Pref(v)
		}
	}
	if opts.Model.IsDynamic() {
		if v, ok := prefs[prefModel]; ok && v != "" {
			opts.Model = cfgpkg.Pref(v)
		}
	}
	if opts.DefaultLayout.IsDynamic() {
		if v, ok := prefs[prefDefaultLayout]; ok && v != "" {
			opts.DefaultLayout = cfgpkg.Pref(v)
		}
	}
	if opts.Mode.IsDynamic() {
		if v, ok := prefs[prefMode]; ok && v != "" {
			opts.Mode = cfgpkg.Pref(v)
		}
	}
	if opts.HideThinking == cfgpkg.Unset { // tri-state has no Dynamic; absence = remembered
		// HideThinking persists only when the config left it unset (so the
		// remembered value, not a hardcoded literal, wins).
		if v, ok := prefs[prefHideThinking]; ok {
			opts.HideThinking = boolFromString(v)
		}
	}
	if opts.OutputPercent.IsDynamic() {
		if v, ok := prefs[prefOutputPercent]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				opts.OutputPercent = cfgpkg.Int(n)
			}
		}
	}
	if opts.SplitLeftWidth.IsDynamic() {
		if v, ok := prefs[prefSplitLeft]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				opts.SplitLeftWidth = cfgpkg.Int(n)
			}
		}
	}
	return opts
}

func boolFromString(v string) cfgpkg.Bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		return cfgpkg.True
	case "false":
		return cfgpkg.False
	default:
		return cfgpkg.Unset
	}
}

// DynamicPrefWriter persists runtime preference changes back to state.toml, but
// only for fields the config declared cfg.Dynamic. Non-Dynamic fields are
// pinned by the user and never overwritten by runtime changes.
type DynamicPrefWriter struct {
	path     string
	declared map[string]bool
}

// NewDynamicPrefWriter builds a writer from the resolved Opts: a field is
// persistable only when it was declared Dynamic (Pref/Count) or, for
// HideThinking, left unset so the runtime value is what's remembered.
func NewDynamicPrefWriter(opts cfgpkg.Opts) *DynamicPrefWriter {
	path, err := statePrefsPath()
	if err != nil {
		path = ""
	}
	declared := map[string]bool{
		prefBackend:       opts.Backend.IsDynamic(),
		prefModel:         opts.Model.IsDynamic(),
		prefDefaultLayout: opts.DefaultLayout.IsDynamic(),
		prefMode:          opts.Mode.IsDynamic(),
		prefHideThinking:  opts.HideThinking == cfgpkg.Unset,
		prefOutputPercent: opts.OutputPercent.IsDynamic(),
		prefSplitLeft:     opts.SplitLeftWidth.IsDynamic(),
	}
	return &DynamicPrefWriter{path: path, declared: declared}
}

// Persist merges the provided key→value updates into state.toml, dropping any
// key the config did not declare Dynamic. A no-op (no declared keys, or empty
// updates) skips the disk write.
func (w *DynamicPrefWriter) Persist(updates map[string]string) error {
	if w == nil || w.path == "" {
		return nil
	}
	filtered := map[string]string{}
	for key, value := range updates {
		if w.declared[key] {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	current, err := loadStatePrefs(w.path)
	if err != nil {
		return err
	}
	changed := false
	for key, value := range filtered {
		if current[key] != value {
			current[key] = value
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return writeStatePrefs(w.path, current)
}

// Declares reports whether key is persistable (declared Dynamic by the config).
func (w *DynamicPrefWriter) Declares(key string) bool {
	if w == nil {
		return false
	}
	return w.declared[key]
}
