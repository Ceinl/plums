package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	UserModulePath  = "github.com/Ceinl/plums-user"
	PlumsModulePath = "github.com/Ceinl/plums"
)

type Options struct {
	ConfigDir      string
	OutputPath     string
	WorkDir        string
	PlumsModuleDir string
	PlumsVersion   string
	Version        string
	Commit         string
	BuildDate      string
	Force          bool
	Stdout         io.Writer
	Stderr         io.Writer
}

func Build(ctx context.Context, opts Options) (string, error) {
	configDir, err := resolveConfigDir(opts.ConfigDir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.go")); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("build: %s does not contain config.go", configDir)
		}
		return "", err
	}
	modulePath, err := ModulePath(configDir)
	if err != nil {
		return "", err
	}
	usesConfigModule := fileExists(filepath.Join(configDir, "go.mod"))
	outputPath, cached, err := buildOutputPath(configDir, opts)
	if err != nil {
		return "", err
	}
	if cached && !opts.Force {
		if executableExists(outputPath) {
			return outputPath, nil
		}
	}
	workDir := opts.WorkDir
	if workDir == "" {
		workDir, err = os.MkdirTemp("", "plums-build-*")
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", err
	}
	if err := copyTree(configDir, workDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(workDir, "cmd", "plums"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(workDir, "cmd", "plums", "main.go"), []byte(GenerateMainForModule(opts, modulePath)), 0o644); err != nil {
		return "", err
	}
	if usesConfigModule {
		if err := ensureBuildGoMod(workDir, opts); err != nil {
			return "", err
		}
	} else {
		if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(GenerateGoMod(opts)), 0o644); err != nil {
			return "", err
		}
	}
	if opts.PlumsModuleDir != "" && !usesConfigModule {
		if err := copyGoSum(opts.PlumsModuleDir, workDir); err != nil {
			return "", err
		}
	}
	if err := runGo(ctx, workDir, opts.Stdout, opts.Stderr, "mod", "tidy"); err != nil {
		return "", fmt.Errorf("build: go mod tidy: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", err
	}
	if err := runGo(ctx, workDir, opts.Stdout, opts.Stderr, "build", "-o", outputPath, "./cmd/plums"); err != nil {
		return "", fmt.Errorf("build: go build: %w", err)
	}
	return outputPath, nil
}

func buildOutputPath(configDir string, opts Options) (string, bool, error) {
	if opts.OutputPath != "" {
		outputPath, err := resolveOutputPath(opts.OutputPath)
		return outputPath, false, err
	}
	key, err := CacheKey(configDir, opts)
	if err != nil {
		return "", false, err
	}
	outputPath, err := CachedOutputPath(key)
	return outputPath, true, err
}

func CacheKey(configDir string, opts Options) (string, error) {
	configDir, err := resolveConfigDir(configDir)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	writeHashString(h, "plums-build-cache-v1\n")
	writeHashString(h, "plums-version:"+opts.PlumsVersion+"\n")
	writeHashString(h, "version:"+opts.Version+"\n")
	writeHashString(h, "commit:"+opts.Commit+"\n")
	writeHashString(h, "build-date:"+opts.BuildDate+"\n")
	if err := hashTree(h, configDir, "config"); err != nil {
		return "", err
	}
	if opts.PlumsModuleDir != "" && opts.PlumsVersion == "" {
		writeHashString(h, "plums-module-dir:"+filepath.ToSlash(opts.PlumsModuleDir)+"\n")
		if err := hashTree(h, opts.PlumsModuleDir, "plums"); err != nil {
			return "", err
		}
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:20], nil
}

func CachedOutputPath(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("build: empty cache key")
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "plums", "builds", key, "plums"), nil
}

func hashTree(h hash.Hash, root, label string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.IsDir() && skipBuildArtifactDir(entry.Name()) {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		writeHashString(h, label+":"+filepath.ToSlash(rel)+"\n")
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(h, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		writeHashString(h, "\n")
		return nil
	})
}

func writeHashString(h hash.Hash, value string) {
	_, _ = io.WriteString(h, value)
}

func executableExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func runGo(ctx context.Context, dir string, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func GenerateMain(opts Options) string {
	return GenerateMainForModule(opts, UserModulePath)
}

func GenerateMainForModule(opts Options, modulePath string) string {
	if strings.TrimSpace(modulePath) == "" {
		modulePath = UserModulePath
	}
	return fmt.Sprintf(`package main

import (
	_ %q

	plumsruntime "github.com/Ceinl/plums/plums/runtime"
)

var (
	version = %q
	commit = %q
	buildDate = %q
)

func main() {
	plumsruntime.Main(version, commit, buildDate)
}
`, modulePath, valueOr(opts.Version, "0.1.0-dev"), valueOr(opts.Commit, "unknown"), valueOr(opts.BuildDate, "unknown"))
}

func GenerateGoMod(opts Options) string {
	version := opts.PlumsVersion
	if version == "" {
		version = "v0.0.0"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n\n", UserModulePath)
	b.WriteString("go 1.26.2\n\n")
	fmt.Fprintf(&b, "require %s %s\n", PlumsModulePath, version)
	if opts.PlumsModuleDir != "" {
		fmt.Fprintf(&b, "\nreplace %s => %s\n", PlumsModulePath, filepath.ToSlash(opts.PlumsModuleDir))
	}
	return b.String()
}

func ModulePath(configDir string) (string, error) {
	configDir, err := resolveConfigDir(configDir)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(configDir, "go.mod"))
	if err != nil {
		if os.IsNotExist(err) {
			return UserModulePath, nil
		}
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("build: %s has no module declaration", filepath.Join(configDir, "go.mod"))
}

func ModuleVersion(version string) string {
	if len(version) >= 2 && version[0] == 'v' && version[1] >= '0' && version[1] <= '9' {
		return version
	}
	return ""
}

func ensureBuildGoMod(workDir string, opts Options) error {
	path := filepath.Join(workDir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)
	var extra strings.Builder
	if !strings.Contains(text, PlumsModulePath) {
		version := opts.PlumsVersion
		if version == "" {
			version = "v0.0.0"
		}
		fmt.Fprintf(&extra, "\nrequire %s %s\n", PlumsModulePath, version)
	}
	if opts.PlumsModuleDir != "" && !strings.Contains(text, "replace "+PlumsModulePath) {
		fmt.Fprintf(&extra, "\nreplace %s => %s\n", PlumsModulePath, filepath.ToSlash(opts.PlumsModuleDir))
	}
	if extra.Len() == 0 {
		return nil
	}
	return os.WriteFile(path, append(data, []byte(extra.String())...), 0o644)
}

// TidyConfigDir runs `go mod tidy` in configDir when go.sum is missing, so a
// freshly seeded config module resolves its imports in editors (gopls needs
// go.sum entries) without the user running go commands by hand. Callers treat
// failure as non-fatal: the auto-build path tidies its own work module anyway,
// and an offline first launch should not break seeding.
func TidyConfigDir(ctx context.Context, configDir string, stdout, stderr io.Writer) error {
	configDir, err := resolveConfigDir(configDir)
	if err != nil {
		return err
	}
	if fileExists(filepath.Join(configDir, "go.sum")) {
		return nil
	}
	return runGo(ctx, configDir, stdout, stderr, "mod", "tidy")
}

func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "plums", "config"), nil
}

func DefaultOutputPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "plums", "plums"), nil
}

func resolveConfigDir(path string) (string, error) {
	if path == "" {
		return DefaultConfigDir()
	}
	return filepath.Abs(path)
}

func resolveOutputPath(path string) (string, error) {
	if path == "" {
		return DefaultOutputPath()
	}
	return filepath.Abs(path)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() && skipBuildArtifactDir(name) {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func skipBuildArtifactDir(name string) bool {
	switch name {
	case ".git", "bin", ".cache":
		return true
	default:
		return false
	}
}

func copyGoSum(moduleDir, workDir string) error {
	src := filepath.Join(moduleDir, "go.sum")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return copyFile(src, filepath.Join(workDir, "go.sum"), 0o644)
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
