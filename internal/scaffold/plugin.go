package scaffold

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Ceinl/plums/internal/app/defaults"
	internalbuild "github.com/Ceinl/plums/internal/build"
)

type PluginOptions struct {
	ConfigDir      string
	Name           string
	PlumsVersion   string
	PlumsModuleDir string
}

type PluginResult struct {
	Dir         string
	ImportPath  string
	PackageName string
}

func NewPlugin(opts PluginOptions) (PluginResult, error) {
	configDir, err := resolveConfigDir(opts.ConfigDir)
	if err != nil {
		return PluginResult{}, err
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return PluginResult{}, fmt.Errorf("plugin name is required")
	}
	dirName, packageName, err := pluginNames(name)
	if err != nil {
		return PluginResult{}, err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return PluginResult{}, err
	}
	if err := defaults.WriteAll(configDir, defaults.Options{
		PlumsVersion:   opts.PlumsVersion,
		PlumsModuleDir: opts.PlumsModuleDir,
	}); err != nil {
		return PluginResult{}, err
	}
	modulePath, err := internalbuild.ModulePath(configDir)
	if err != nil {
		return PluginResult{}, err
	}
	target := filepath.Join(configDir, "plugins", dirName)
	if _, err := os.Stat(target); err == nil {
		return PluginResult{}, fmt.Errorf("plugin %q already exists at %s", name, target)
	} else if !os.IsNotExist(err) {
		return PluginResult{}, err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return PluginResult{}, err
	}
	if err := writeGoFile(filepath.Join(target, packageName+".go"), pluginSource(dirName, packageName)); err != nil {
		return PluginResult{}, err
	}
	if err := writeGoFile(filepath.Join(target, packageName+"_test.go"), pluginTestSource(packageName)); err != nil {
		return PluginResult{}, err
	}
	return PluginResult{
		Dir:         target,
		ImportPath:  modulePath + "/plugins/" + dirName,
		PackageName: packageName,
	}, nil
}

func resolveConfigDir(path string) (string, error) {
	if path != "" {
		return filepath.Abs(path)
	}
	return internalbuild.DefaultConfigDir()
}

func pluginNames(name string) (dirName, packageName string, err error) {
	dirName = strings.ToLower(strings.TrimSpace(name))
	for _, r := range dirName {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return "", "", fmt.Errorf("plugin name %q may contain only letters, numbers, '-' and '_'", name)
	}
	packageName = packageIdent(dirName)
	if packageName == "" {
		return "", "", fmt.Errorf("plugin name %q does not contain a valid package identifier", name)
	}
	return dirName, packageName, nil
}

func packageIdent(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '-' || r == '_':
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	first, _ := utf8.DecodeRuneInString(out)
	if !unicode.IsLetter(first) {
		out = "plugin_" + out
	}
	if goKeywords[out] {
		out = "plugin_" + out
	}
	return out
}

var goKeywords = map[string]bool{
	"break":       true,
	"default":     true,
	"func":        true,
	"interface":   true,
	"select":      true,
	"case":        true,
	"defer":       true,
	"go":          true,
	"map":         true,
	"struct":      true,
	"chan":        true,
	"else":        true,
	"goto":        true,
	"package":     true,
	"switch":      true,
	"const":       true,
	"fallthrough": true,
	"if":          true,
	"range":       true,
	"type":        true,
	"continue":    true,
	"for":         true,
	"import":      true,
	"return":      true,
	"var":         true,
}

func writeGoFile(path, source string) error {
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	return os.WriteFile(path, formatted, 0o644)
}

func pluginSource(pluginName, packageName string) string {
	commandName := strings.ReplaceAll(packageName, "_", ".") + ".say"
	return fmt.Sprintf(`package %s

import (
	"context"
	"fmt"

	"github.com/Ceinl/plums/capabilities"
	cfg "github.com/Ceinl/plums/config"
)

type Options struct {
	Message string
}

func New(opts Options) cfg.Plugin {
	return cfg.Plugin{Self: &Plugin{}, Opts: opts}
}

type Plugin struct {
	opts Options
}

func (*Plugin) Name() string { return %q }

func (p *Plugin) Init(_ capabilities.Host, raw any) error {
	if raw != nil {
		opts, ok := raw.(Options)
		if !ok {
			return fmt.Errorf("%s: expected Options, got %%T", raw)
		}
		p.opts = opts
	}
	if p.opts.Message == "" {
		p.opts.Message = "hello from %s"
	}
	return nil
}

func (p *Plugin) Commands() []capabilities.Command {
	return []capabilities.Command{{
		Name:   %q,
		Detail: "Write a test message",
		Do: func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.Chat("system", p.opts.Message)
			return nil
		},
	}}
}
`, packageName, pluginName, pluginName, pluginName, commandName)
}

func pluginTestSource(packageName string) string {
	return fmt.Sprintf(`package %s

import (
	"testing"

	"github.com/Ceinl/plums/plugintest"
)

func TestPlugin(t *testing.T) {
	plugintest.Check(t, New(Options{}))
}
`, packageName)
}
