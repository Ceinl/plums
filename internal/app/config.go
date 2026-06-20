package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Ceinl/plums/internal/app/defaults"
)

// InitGlobalConfig creates the default config files in ~/.config/plums/config.
func InitGlobalConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "plums", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := defaults.WriteAll(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// UserConfigGoPath returns the path of the compiled user config source,
// ~/.config/plums/config/config.go. This is the only user-facing authoring
// surface; auto-build compiles it into the running binary.
func UserConfigGoPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "plums", "config", "config.go"), nil
}

// ResolveConfigPath reports the global config directory,
// ~/.config/plums/config. plums is global-only (neovim style): there is no
// project-local config. When the directory does not exist, "" is returned so
// callers fall back to the built-in defaults (works out of the box).
func ResolveConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "plums", "config")
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return dir, nil
}

// ValidBackendProvider reports whether provider names a backend plums ships.
func ValidBackendProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "opencode", "codex", "claude", "claude-mirror":
		return true
	default:
		return false
	}
}
