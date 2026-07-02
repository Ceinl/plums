package scaffold

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

type ModuleOptions struct {
	ConfigDir      string
	PlumsVersion   string
	PlumsModuleDir string
	Stdout         io.Writer
	Stderr         io.Writer
}

func AddModule(ctx context.Context, opts ModuleOptions, target string) (string, error) {
	configDir, err := EnsureConfig(ConfigOptions{
		ConfigDir:      opts.ConfigDir,
		PlumsVersion:   opts.PlumsVersion,
		PlumsModuleDir: opts.PlumsModuleDir,
	})
	if err != nil {
		return "", err
	}
	if target == "" {
		return "", fmt.Errorf("module path is required")
	}
	if err := runGo(ctx, configDir, opts.Stdout, opts.Stderr, "get", target); err != nil {
		return "", fmt.Errorf("go get %s: %w", target, err)
	}
	if err := runGo(ctx, configDir, opts.Stdout, opts.Stderr, "mod", "tidy"); err != nil {
		return "", fmt.Errorf("go mod tidy: %w", err)
	}
	return configDir, nil
}

func UpdateModules(ctx context.Context, opts ModuleOptions, targets []string) (string, error) {
	configDir, err := EnsureConfig(ConfigOptions{
		ConfigDir:      opts.ConfigDir,
		PlumsVersion:   opts.PlumsVersion,
		PlumsModuleDir: opts.PlumsModuleDir,
	})
	if err != nil {
		return "", err
	}
	args := []string{"get", "-u"}
	if len(targets) == 0 {
		args = append(args, "./...")
	} else {
		args = append(args, targets...)
	}
	if err := runGo(ctx, configDir, opts.Stdout, opts.Stderr, args...); err != nil {
		return "", fmt.Errorf("go %v: %w", args, err)
	}
	if err := runGo(ctx, configDir, opts.Stdout, opts.Stderr, "mod", "tidy"); err != nil {
		return "", fmt.Errorf("go mod tidy: %w", err)
	}
	return configDir, nil
}

func runGo(ctx context.Context, dir string, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
