package runtime

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	cfgpkg "github.com/Ceinl/plums/config"
	"github.com/Ceinl/plums/internal/api"
	"github.com/Ceinl/plums/internal/app"
	internalbuild "github.com/Ceinl/plums/internal/build"
	"github.com/Ceinl/plums/internal/scaffold"
)

// Config holds process-level launch options for a plums binary.
type Config = api.Config

// autobuildEnv guards against an auto-build re-exec loop.
const autobuildEnv = "PLUMS_AUTOBUILD"

// DefaultConfig returns a Config populated with built-in defaults.
func DefaultConfig() *Config {
	return api.DefaultConfig()
}

// RegisterFlags binds CLI flags to the supplied Config.
func RegisterFlags(cfg *Config) {
	api.RegisterFlags(cfg)
}

// Run starts plums using the supplied launch config. If cfg is nil, defaults
// are used.
func Run(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return api.Run(cfg)
}

// Main is the standard entrypoint for stock and generated plums binaries.
func Main(version, commit, buildDate string) {
	if len(os.Args) > 1 && isBuildCommand(os.Args[1]) {
		runBuild(os.Args[2:], version, commit, buildDate, os.Args[1] != "build")
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "plugin" {
		runPlugin(os.Args[2:], version)
		return
	}

	// neovim-style: a stock binary that finds a user config.go in
	// ~/.config/plums/config compiles it (cached by content hash) and runs the
	// personalized binary. The compiled binary registers its config via
	// config.Use, so it skips this branch and runs directly — no recursion.
	noConfig := hasNoConfigArg(os.Args[1:])
	if _, registered := cfgpkg.Registered(); !registered && !noConfig && os.Getenv(autobuildEnv) == "" {
		if ran, err := runUserConfig(version, commit, buildDate); err != nil {
			fmt.Fprintf(os.Stderr, "plums: user config build failed (%v); running defaults\n", err)
		} else if ran {
			return
		}
	}

	cfg := DefaultConfig()
	cfg.Version = version
	cfg.Commit = commit
	cfg.BuildDate = buildDate
	cfg.NoConfig = noConfig

	RegisterFlags(cfg)
	flag.Parse()

	if err := Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "plums: %v\n", err)
		os.Exit(1)
	}
}

// runUserConfig compiles ~/.config/plums/config/config.go (if present) and execs
// the resulting binary, forwarding args and exit code. It reports whether the
// compiled binary was run; (false, nil) means "no user config — run defaults".
func runUserConfig(version, commit, buildDate string) (bool, error) {
	configDir, err := internalbuild.DefaultConfigDir()
	if err != nil {
		return false, nil
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.go")); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		plumsVersion, plumsDir := configBuildSource(version)
		if _, err := app.InitGlobalConfig(app.InitConfigOptions{
			PlumsVersion:   plumsVersion,
			PlumsModuleDir: plumsDir,
		}); err != nil {
			return false, err
		}
	}
	plumsVersion, plumsDir := configBuildSource(version)
	bin, err := internalbuild.Build(context.Background(), internalbuild.Options{
		ConfigDir: configDir,
		// A released binary (version like "v1.2.3") tells the build which plums
		// module version to require so `go mod tidy` resolves it. A dev build
		// ("0.1.0-dev") uses a best-effort local checkout replace when the
		// source path is available; otherwise the build fails and stock defaults
		// run as the broken-config fallback.
		PlumsVersion:   plumsVersion,
		PlumsModuleDir: plumsDir,
		Version:        version,
		Commit:         commit,
		BuildDate:      buildDate,
		Stdout:         os.Stderr,
		Stderr:         os.Stderr,
	})
	if err != nil {
		return false, err
	}
	if self, err := os.Executable(); err == nil && sameFile(self, bin) {
		return false, nil
	}
	cmd := exec.Command(bin, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), autobuildEnv+"=1")
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return false, err
	}
	return true, nil
}

func sameFile(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

func configBuildSource(version string) (plumsVersion, plumsDir string) {
	plumsVersion = internalbuild.ModuleVersion(version)
	if plumsVersion != "" {
		return plumsVersion, ""
	}
	return "", localPlumsModuleDir()
}

func localPlumsModuleDir() string {
	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		return ""
	}
	dir := filepath.Dir(file)
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(data), "module "+internalbuild.PlumsModulePath) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func hasNoConfigArg(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--no-config" || arg == "-no-config" {
			return true
		}
		if strings.HasPrefix(arg, "--no-config=") {
			value := strings.TrimPrefix(arg, "--no-config=")
			return value == "" || value == "1" || value == "true"
		}
		if strings.HasPrefix(arg, "-no-config=") {
			value := strings.TrimPrefix(arg, "-no-config=")
			return value == "" || value == "1" || value == "true"
		}
	}
	return false
}

func isBuildCommand(arg string) bool {
	switch arg {
	case "build", "--recompile", "-recompile":
		return true
	default:
		return false
	}
}

func runBuild(args []string, version, commit, buildDate string, force bool) {
	fs := flag.NewFlagSet("plums build", flag.ExitOnError)
	configDir := fs.String("config-dir", "", "directory containing config.go (default ~/.config/plums/config)")
	outputPath := fs.String("o", "", "compiled binary output path")
	outputPathLong := fs.String("output", "", "compiled binary output path")
	forceBuild := fs.Bool("force", force, "rebuild even when the source hash is already cached")
	workDir := fs.String("work-dir", "", "temporary build module directory")
	plumsDir := fs.String("plums-dir", "", "local github.com/Ceinl/plums module checkout for a replace directive")
	plumsVersion := fs.String("plums-version", "", "github.com/Ceinl/plums module version to require")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "plums build: %v\n", err)
		os.Exit(2)
	}
	if *outputPath == "" {
		*outputPath = *outputPathLong
	}
	output, err := internalbuild.Build(context.Background(), internalbuild.Options{
		ConfigDir:      *configDir,
		OutputPath:     *outputPath,
		WorkDir:        *workDir,
		PlumsModuleDir: *plumsDir,
		PlumsVersion:   *plumsVersion,
		Version:        version,
		Commit:         commit,
		BuildDate:      buildDate,
		Force:          *forceBuild,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "plums build: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "built plums %s\n", output)
}

func runPlugin(args []string, version string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: plums plugin <new|add|update|list> ...")
		os.Exit(2)
	}
	switch args[0] {
	case "new":
		runPluginNew(args[1:], version)
	case "add":
		runPluginAdd(args[1:], version)
	case "update":
		runPluginUpdate(args[1:], version)
	case "list":
		runPluginList(args[1:], version)
	default:
		fmt.Fprintf(os.Stderr, "plums plugin: unknown command %q\n", args[0])
		os.Exit(2)
	}
}

func runPluginNew(args []string, version string) {
	fs := flag.NewFlagSet("plums plugin new", flag.ExitOnError)
	configDir := fs.String("config-dir", "", "config module directory (default ~/.config/plums/config)")
	defaultPlumsVersion, defaultPlumsDir := configBuildSource(version)
	plumsDir := fs.String("plums-dir", defaultPlumsDir, "local github.com/Ceinl/plums module checkout for a replace directive")
	plumsVersion := fs.String("plums-version", defaultPlumsVersion, "github.com/Ceinl/plums module version to require")
	noWire := fs.Bool("no-wire", false, "create files but do not add the plugin to config.go")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "plums plugin new: %v\n", err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: plums plugin new <name>")
		os.Exit(2)
	}
	result, err := scaffold.NewPlugin(scaffold.PluginOptions{
		ConfigDir:      *configDir,
		Name:           fs.Arg(0),
		PlumsVersion:   *plumsVersion,
		PlumsModuleDir: *plumsDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "plums plugin new: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "created plugin %s\n", result.Dir)
	if !*noWire {
		wire, err := scaffold.WirePlugin(scaffold.WireOptions{
			ConfigDir:   *configDir,
			ImportPath:  result.ImportPath,
			PackageName: result.PackageName,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "plums plugin new: %v\n", err)
		} else if wire.Changed {
			fmt.Fprintf(os.Stdout, "wired %s into %s\n", wire.Expression, wire.ConfigPath)
			return
		} else {
			fmt.Fprintf(os.Stdout, "%s already wired in %s\n", wire.Expression, wire.ConfigPath)
			return
		}
	}
	fmt.Fprintf(os.Stdout, "\nAdd it to config.go:\n\n")
	fmt.Fprintf(os.Stdout, "import %s %q\n\n", result.PackageName, result.ImportPath)
	fmt.Fprintf(os.Stdout, "%s.New(%s.Options{})\n", result.PackageName, result.PackageName)
}

func runPluginAdd(args []string, version string) {
	fs := flag.NewFlagSet("plums plugin add", flag.ExitOnError)
	configDir := fs.String("config-dir", "", "config module directory (default ~/.config/plums/config)")
	defaultPlumsVersion, defaultPlumsDir := configBuildSource(version)
	plumsDir := fs.String("plums-dir", defaultPlumsDir, "local github.com/Ceinl/plums module checkout for a replace directive")
	plumsVersion := fs.String("plums-version", defaultPlumsVersion, "github.com/Ceinl/plums module version to require")
	noWire := fs.Bool("no-wire", false, "download module but do not add the plugin to config.go")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "plums plugin add: %v\n", err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: plums plugin add <go-import-path[@version]>")
		os.Exit(2)
	}
	target := fs.Arg(0)
	if _, err := scaffold.AddModule(context.Background(), scaffold.ModuleOptions{
		ConfigDir:      *configDir,
		PlumsVersion:   *plumsVersion,
		PlumsModuleDir: *plumsDir,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
	}, target); err != nil {
		fmt.Fprintf(os.Stderr, "plums plugin add: %v\n", err)
		os.Exit(1)
	}
	importPath := scaffold.ImportPathForGetTarget(target)
	packageName := scaffold.PackageNameForImportPath(importPath)
	fmt.Fprintf(os.Stdout, "added plugin module %s\n", target)
	if !*noWire {
		wire, err := scaffold.WirePlugin(scaffold.WireOptions{
			ConfigDir:   *configDir,
			ImportPath:  importPath,
			PackageName: packageName,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "plums plugin add: %v\n", err)
		} else if wire.Changed {
			fmt.Fprintf(os.Stdout, "wired %s into %s\n", wire.Expression, wire.ConfigPath)
			return
		} else {
			fmt.Fprintf(os.Stdout, "%s already wired in %s\n", wire.Expression, wire.ConfigPath)
			return
		}
	}
	fmt.Fprintf(os.Stdout, "\nAdd it to config.go:\n\n")
	fmt.Fprintf(os.Stdout, "import %s %q\n\n", packageName, importPath)
	fmt.Fprintf(os.Stdout, "%s.New(%s.Options{})\n", packageName, packageName)
}

func runPluginUpdate(args []string, version string) {
	fs := flag.NewFlagSet("plums plugin update", flag.ExitOnError)
	configDir := fs.String("config-dir", "", "config module directory (default ~/.config/plums/config)")
	defaultPlumsVersion, defaultPlumsDir := configBuildSource(version)
	plumsDir := fs.String("plums-dir", defaultPlumsDir, "local github.com/Ceinl/plums module checkout for a replace directive")
	plumsVersion := fs.String("plums-version", defaultPlumsVersion, "github.com/Ceinl/plums module version to require")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "plums plugin update: %v\n", err)
		os.Exit(2)
	}
	if _, err := scaffold.UpdateModules(context.Background(), scaffold.ModuleOptions{
		ConfigDir:      *configDir,
		PlumsVersion:   *plumsVersion,
		PlumsModuleDir: *plumsDir,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
	}, fs.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "plums plugin update: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "updated plugin dependencies")
}

func runPluginList(args []string, version string) {
	fs := flag.NewFlagSet("plums plugin list", flag.ExitOnError)
	configDir := fs.String("config-dir", "", "config module directory (default ~/.config/plums/config)")
	defaultPlumsVersion, defaultPlumsDir := configBuildSource(version)
	plumsDir := fs.String("plums-dir", defaultPlumsDir, "local github.com/Ceinl/plums module checkout for a replace directive")
	plumsVersion := fs.String("plums-version", defaultPlumsVersion, "github.com/Ceinl/plums module version to require")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "plums plugin list: %v\n", err)
		os.Exit(2)
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: plums plugin list")
		os.Exit(2)
	}
	plugins, err := scaffold.ListPlugins(scaffold.ConfigOptions{
		ConfigDir:      *configDir,
		PlumsVersion:   *plumsVersion,
		PlumsModuleDir: *plumsDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "plums plugin list: %v\n", err)
		os.Exit(1)
	}
	if len(plugins) == 0 {
		fmt.Fprintln(os.Stdout, "no plugins found")
		return
	}
	for _, plugin := range plugins {
		kind := "module"
		if plugin.Local {
			kind = "local"
		}
		status := "not wired"
		if plugin.Wired {
			status = "wired"
		}
		fmt.Fprintf(os.Stdout, "%-8s %-10s %s\n", kind, status, plugin.ImportPath)
	}
}
