package scaffold

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Ceinl/plums/internal/app/defaults"
	internalbuild "github.com/Ceinl/plums/internal/build"
)

type ConfigOptions struct {
	ConfigDir      string
	PlumsVersion   string
	PlumsModuleDir string
}

type WireOptions struct {
	ConfigDir   string
	ImportPath  string
	PackageName string
}

type WireResult struct {
	ConfigPath string
	Changed    bool
	Expression string
}

type ListedPlugin struct {
	Name       string
	ImportPath string
	Dir        string
	Local      bool
	Wired      bool
}

func EnsureConfig(opts ConfigOptions) (string, error) {
	configDir, err := resolveConfigDir(opts.ConfigDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", err
	}
	if err := defaults.WriteAll(configDir, defaults.Options{
		PlumsVersion:   opts.PlumsVersion,
		PlumsModuleDir: opts.PlumsModuleDir,
	}); err != nil {
		return "", err
	}
	return configDir, nil
}

func WirePlugin(opts WireOptions) (WireResult, error) {
	configDir, err := resolveConfigDir(opts.ConfigDir)
	if err != nil {
		return WireResult{}, err
	}
	path := filepath.Join(configDir, "config.go")
	data, err := os.ReadFile(path)
	if err != nil {
		return WireResult{}, err
	}
	importPath := strings.TrimSpace(opts.ImportPath)
	if importPath == "" {
		return WireResult{}, fmt.Errorf("plugin import path is required")
	}
	packageName := strings.TrimSpace(opts.PackageName)
	if packageName == "" {
		packageName = PackageNameForImportPath(importPath)
	}
	expr := fmt.Sprintf("%s.New(%s.Options{})", packageName, packageName)
	text := string(data)
	changed := false
	var warnings []string
	if !hasImport(text, importPath) {
		var ok bool
		text, ok = addImport(text, packageName, importPath)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("import %s %q", packageName, importPath))
		} else {
			changed = true
		}
	}
	if !strings.Contains(text, expr) {
		var ok bool
		text, ok = addPluginExpression(text, expr)
		if !ok {
			warnings = append(warnings, expr)
		} else {
			changed = true
		}
	}
	if len(warnings) > 0 {
		return WireResult{ConfigPath: path, Changed: changed, Expression: expr}, fmt.Errorf("could not wire config.go automatically; add manually: %s", strings.Join(warnings, "; "))
	}
	formatted, err := format.Source([]byte(text))
	if err != nil {
		return WireResult{}, fmt.Errorf("format config.go after wiring: %w", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), path, formatted, parser.AllErrors); err != nil {
		return WireResult{}, fmt.Errorf("parse config.go after wiring: %w", err)
	}
	if !bytes.Equal(data, formatted) {
		changed = true
	}
	if changed {
		if err := os.WriteFile(path, formatted, 0o644); err != nil {
			return WireResult{}, err
		}
	}
	return WireResult{ConfigPath: path, Changed: changed, Expression: expr}, nil
}

func ListPlugins(opts ConfigOptions) ([]ListedPlugin, error) {
	configDir, err := EnsureConfig(opts)
	if err != nil {
		return nil, err
	}
	modulePath, err := internalbuild.ModulePath(configDir)
	if err != nil {
		return nil, err
	}
	configData, _ := os.ReadFile(filepath.Join(configDir, "config.go"))
	configText := string(configData)
	var plugins []ListedPlugin
	localRoot := filepath.Join(configDir, "plugins")
	entries, err := os.ReadDir(localRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		importPath := modulePath + "/plugins/" + name
		packageName := PackageNameForImportPath(importPath)
		plugins = append(plugins, ListedPlugin{
			Name:       name,
			ImportPath: importPath,
			Dir:        filepath.Join(localRoot, name),
			Local:      true,
			Wired:      hasImport(configText, importPath) && strings.Contains(configText, packageName+".New("),
		})
	}
	external := directRequireModules(configDir)
	for _, module := range external {
		if module == internalbuild.PlumsModulePath || strings.HasPrefix(module, modulePath+"/plugins/") {
			continue
		}
		packageName := PackageNameForImportPath(module)
		plugins = append(plugins, ListedPlugin{
			Name:       filepath.Base(module),
			ImportPath: module,
			Local:      false,
			Wired:      hasImport(configText, module) && strings.Contains(configText, packageName+".New("),
		})
	}
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Local != plugins[j].Local {
			return plugins[i].Local
		}
		return plugins[i].ImportPath < plugins[j].ImportPath
	})
	return plugins, nil
}

func PackageNameForImportPath(importPath string) string {
	path := strings.TrimSpace(importPath)
	if at := strings.IndexByte(path, '@'); at >= 0 {
		path = path[:at]
	}
	path = strings.TrimSuffix(path, "/")
	base := filepath.Base(path)
	if isMajorVersionPath(base) {
		parent := filepath.Base(filepath.Dir(path))
		if parent != "." && parent != "/" {
			base = parent
		}
	}
	return packageIdent(base)
}

func ImportPathForGetTarget(target string) string {
	target = strings.TrimSpace(target)
	if at := strings.IndexByte(target, '@'); at >= 0 {
		return target[:at]
	}
	return target
}

func hasImport(source, importPath string) bool {
	file, err := parser.ParseFile(token.NewFileSet(), "config.go", source, parser.ImportsOnly)
	if err != nil {
		return strings.Contains(source, strconvQuote(importPath))
	}
	quoted := strconvQuote(importPath)
	for _, spec := range file.Imports {
		if spec.Path != nil && spec.Path.Value == quoted {
			return true
		}
	}
	return false
}

func addImport(source, packageName, importPath string) (string, bool) {
	blockRe := regexp.MustCompile(`(?m)^import\s*\(\n`)
	if loc := blockRe.FindStringIndex(source); loc != nil {
		close := strings.Index(source[loc[1]:], "\n)")
		if close < 0 {
			return source, false
		}
		insert := loc[1] + close
		line := fmt.Sprintf("\t%s %q", packageName, importPath)
		return source[:insert] + "\n" + line + source[insert:], true
	}
	singleRe := regexp.MustCompile(`(?m)^import\s+((?:[A-Za-z_][A-Za-z0-9_]*\s+)?"[^"]+")\n`)
	if loc := singleRe.FindStringSubmatchIndex(source); loc != nil {
		oldImport := source[loc[2]:loc[3]]
		block := fmt.Sprintf("import (\n\t%s\n\t%s %q\n)\n", oldImport, packageName, importPath)
		return source[:loc[0]] + block + source[loc[1]:], true
	}
	return source, false
}

func addPluginExpression(source, expr string) (string, bool) {
	re := regexp.MustCompile(`(?m)^(\s*)Plugins:\s*\[\][A-Za-z0-9_\.]+\.Plugin\s*\{\s*$`)
	loc := re.FindStringSubmatchIndex(source)
	if loc == nil {
		return source, false
	}
	lineEnd := loc[1]
	indent := source[loc[2]:loc[3]]
	insert := fmt.Sprintf("\n%s\t%s,", indent, expr)
	return source[:lineEnd] + insert + source[lineEnd:], true
}

func directRequireModules(configDir string) []string {
	data, err := os.ReadFile(filepath.Join(configDir, "go.mod"))
	if err != nil {
		return nil
	}
	var modules []string
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "require (") {
			inBlock = true
			continue
		}
		if inBlock && line == ")" {
			inBlock = false
			continue
		}
		if strings.Contains(line, "// indirect") {
			continue
		}
		if strings.HasPrefix(line, "require ") {
			fields := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(fields) >= 1 {
				modules = append(modules, fields[0])
			}
			continue
		}
		if inBlock {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				modules = append(modules, fields[0])
			}
		}
	}
	return modules
}

func isMajorVersionPath(value string) bool {
	if len(value) < 2 || value[0] != 'v' {
		return false
	}
	for _, r := range value[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func strconvQuote(value string) string {
	return strconv.Quote(value)
}
