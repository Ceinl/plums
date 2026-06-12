package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// ResolveConfigPath picks the layout config path based on CLI flags. Global is
// the default so zero-value callers behave like passing --config-global/-cg.
func ResolveConfigPath(global, local bool) (string, error) {
	if local {
		return "./.agents/plums/config/layout.json", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "plums", "config", "layout.json"), nil
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
	case "opencode", "codex", "claude", "claude-mirror":
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

// LoadDefaultLayout reads default_layout from the TOML file at path.
func LoadDefaultLayout(path, fallback string) (string, error) {
	if fallback == "" {
		fallback = "split"
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return "", err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "default_layout" {
			continue
		}
		val, err := parseTomlString(value)
		if err != nil {
			return fallback, nil
		}
		val = strings.ToLower(strings.TrimSpace(val))
		if val == "" {
			return fallback, nil
		}
		return val, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return fallback, nil
}

// LoadHideThinking reads hide_thinking from the TOML file at path.
func LoadHideThinking(path string, fallback bool) bool {
	return loadBoolKey(path, "hide_thinking", fallback)
}

// LoadClearHistory reads clear_history from the TOML file at path.
func LoadClearHistory(path string, fallback bool) bool {
	return loadBoolKey(path, "clear_history", fallback)
}

// loadBoolKey reads a top-level boolean key from the TOML file at path.
func loadBoolKey(path, wantKey string, fallback bool) bool {
	file, err := os.Open(path)
	if err != nil {
		return fallback
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != wantKey {
			continue
		}
		v := strings.ToLower(strings.TrimSpace(value))
		return v == "true"
	}
	return fallback
}

// LoadSplitLeftWidth reads split.left_width from the TOML file at path.
func LoadSplitLeftWidth(path string, fallback int) int {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback
		}
		return fallback
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
		if section != "split" || strings.TrimSpace(key) != "left_width" {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fallback
		}
		return v
	}
	return fallback
}

// SaveConfigValues writes layout, hide_thinking and split.left_width back to
// the TOML file at path, preserving comments and unknown keys. If the file
// does not exist a minimal default file is created.
func SaveConfigValues(path string, layout string, hideThinking bool, leftWidth int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// File doesn't exist — create a minimal TOML with the current values.
		content := fmt.Sprintf(
			"default_layout = %q\nhide_thinking = %t\n\n[split]\nleft_width = %d\n",
			layout, hideThinking, leftWidth,
		)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		return os.WriteFile(path, []byte(content), 0o644)
	}
	lines := strings.Split(string(data), "\n")
	section := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		switch {
		case key == "default_layout" && section == "":
			lines[i] = fmt.Sprintf(`default_layout = "%s"`, layout)
		case key == "hide_thinking" && section == "":
			lines[i] = fmt.Sprintf("hide_thinking = %t", hideThinking)
		case key == "left_width" && section == "split":
			lines[i] = fmt.Sprintf("left_width = %d", leftWidth)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
