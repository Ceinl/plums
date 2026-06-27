package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Ceinl/plums/internal/app/defaults"
)

type InitConfigOptions struct {
	PlumsVersion   string
	PlumsModuleDir string
}

// InitGlobalConfig creates the default config files in ~/.config/plums/config.
func InitGlobalConfig(opts ...InitConfigOptions) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "plums", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	options := defaults.Options{}
	if len(opts) > 0 {
		options.PlumsVersion = opts[0].PlumsVersion
		options.PlumsModuleDir = opts[0].PlumsModuleDir
	}
	if err := defaults.WriteAll(dir, options); err != nil {
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

// ValidBackendProvider reports whether provider names a backend plums ships.
func ValidBackendProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "opencode", "codex", "claude", "claude-mirror":
		return true
	default:
		return false
	}
}
