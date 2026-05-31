package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ceinl/plums/internal/app/defaults"
	"github.com/Ceinl/plums/internal/core/adapter"
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

// InitLocalConfigFiles creates the default config files in ./.agents/plums/config.
func InitLocalConfigFiles() (string, error) {
	dir := filepath.Join(".", ".agents", "plums", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := defaults.WriteAll(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// ResolveConfigPath picks the layout config path based on CLI flags.
func ResolveConfigPath(global, local bool) (string, error) {
	if global && local {
		return "", fmt.Errorf("use only one of --config-global/-cg or --config-local/-cl")
	}
	if local {
		return "./.agents/plums/config/layout.json", nil
	}
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "plums", "config", "layout.json"), nil
	}
	return "", nil
}

// ResolveCommandsConfigPath derives the commands config path from the layout
// config path.
func ResolveCommandsConfigPath(layoutConfigPath string) (string, error) {
	if layoutConfigPath == "" {
		return "", nil
	}
	path := strings.TrimSuffix(layoutConfigPath, "layout.json") + "commands.json"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return path, nil
}

// ResolveOpencodeConfigPath derives the opencode TOML config path from the
// layout config path.
func ResolveOpencodeConfigPath(layoutConfigPath string) string {
	if layoutConfigPath == "" {
		return ".agents/plums/config/config.toml"
	}
	return filepath.Join(filepath.Dir(layoutConfigPath), "config.toml")
}

// LoadOpencodeServerURL reads the opencode_server_url key from the TOML file
// at path. If the file does not exist, fallback is returned. If the file exists
// but the key is missing, fallback is also returned.
func LoadOpencodeServerURL(path, fallback string) (string, error) {
	if fallback == "" {
		fallback = adapter.DefaultBaseURL
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return "", err
	}
	defer func() { _ = file.Close() }()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "opencode_server_url" || section != "opencode" {
			continue
		}
		url, err := parseTomlString(value)
		if err != nil {
			return "", fmt.Errorf("%s opencode.opencode_server_url: %w", path, err)
		}
		if url == "" {
			return "", fmt.Errorf("%s opencode.opencode_server_url is empty", path)
		}
		return url, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return fallback, nil
}

// LoadBackendProvider reads backend.provider from the TOML file at path. If the
// file or key is missing, fallback is returned.
func LoadBackendProvider(path, fallback string) (string, error) {
	if fallback == "" {
		fallback = "opencode"
	}
	fallback = strings.ToLower(strings.TrimSpace(fallback))
	if !validBackendProvider(fallback) {
		return "", fmt.Errorf("unsupported backend provider %q", fallback)
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return "", err
	}
	defer func() { _ = file.Close() }()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "provider" || section != "backend" {
			continue
		}
		provider, err := parseTomlString(value)
		if err != nil {
			return "", fmt.Errorf("%s backend.provider: %w", path, err)
		}
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			return "", fmt.Errorf("%s backend.provider is empty", path)
		}
		if !validBackendProvider(provider) {
			return "", fmt.Errorf("%s backend.provider %q is unsupported", path, provider)
		}
		return provider, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return fallback, nil
}

func validBackendProvider(provider string) bool {
	switch provider {
	case "opencode", "codex":
		return true
	default:
		return false
	}
}

func parseTomlString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '"' {
		return "", fmt.Errorf("expected quoted string")
	}
	// Find the closing quote, skipping escaped quotes.
	end := -1
	for i := 1; i < len(value); i++ {
		if value[i] == '"' && value[i-1] != '\\' {
			end = i
			break
		}
	}
	if end < 0 {
		return "", fmt.Errorf("expected quoted string")
	}
	return strings.TrimSpace(value[1:end]), nil
}
