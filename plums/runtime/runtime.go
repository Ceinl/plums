package runtime

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	cfgpkg "github.com/Ceinl/plums/config"
	"github.com/Ceinl/plums/internal/api"
	internalbuild "github.com/Ceinl/plums/internal/build"
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

	// neovim-style: a stock binary that finds a user config.go in
	// ~/.config/plums/config compiles it (cached by content hash) and runs the
	// personalized binary. The compiled binary registers its config via
	// config.Use, so it skips this branch and runs directly — no recursion.
	if _, registered := cfgpkg.Registered(); !registered && os.Getenv(autobuildEnv) == "" {
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
		return false, nil
	}
	bin, err := internalbuild.Build(context.Background(), internalbuild.Options{
		ConfigDir: configDir,
		// A released binary (version like "v1.2.3") tells the build which plums
		// module version to require so `go mod tidy` resolves it. A dev build
		// ("0.1.0-dev") leaves this empty; the build then fails to resolve and we
		// fall back to stock — use `plums build -plums-dir <checkout>` for dev.
		PlumsVersion: moduleVersion(version),
		Version:      version,
		Commit:       commit,
		BuildDate:    buildDate,
		Stdout:       os.Stderr,
		Stderr:       os.Stderr,
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

// moduleVersion returns version if it is shaped like a Go module version
// ("vX..."), else "" so the build does not pin an unresolvable pseudo-version.
func moduleVersion(version string) string {
	if len(version) >= 2 && version[0] == 'v' && version[1] >= '0' && version[1] <= '9' {
		return version
	}
	return ""
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
