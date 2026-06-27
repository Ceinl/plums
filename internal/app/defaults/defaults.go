package defaults

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed config.go.tmpl
var files embed.FS

const (
	configGo = "config.go"
	goMod    = "go.mod"

	defaultUserModulePath  = "github.com/Ceinl/plums-user"
	defaultPlumsModulePath = "github.com/Ceinl/plums"
	defaultGoVersion       = "1.26.2"
)

type Options struct {
	PlumsVersion   string
	PlumsModuleDir string
}

// Read returns the content of a built-in default config file by name.
func Read(name string) ([]byte, error) {
	return files.ReadFile(sourceName(name))
}

// WriteDefault writes a built-in default file into dir if it does not already
// exist. name must be "config.go" or "go.mod".
func WriteDefault(dir, name string, opts ...Options) error {
	dst := filepath.Join(dir, name)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := defaultFile(name, firstOptions(opts))
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// WriteAll writes every default config file into dir that is not already
// present, leaving existing files untouched. This makes `-init-config`
// idempotent: re-running it on an older config dir adds newly-introduced
// defaults without clobbering edits or erroring on the files that already
// exist. The config directory is a real Go module, so editors and `go test`
// work without users running `go mod init`.
func WriteAll(dir string, opts ...Options) error {
	options := firstOptions(opts)
	for _, name := range []string{goMod, configGo} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := WriteDefault(dir, name, options); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		return err
	}
	return nil
}

func sourceName(name string) string {
	if name == configGo {
		return "config.go.tmpl"
	}
	return name
}

func defaultFile(name string, opts Options) ([]byte, error) {
	if name == goMod {
		return []byte(DefaultGoMod(opts)), nil
	}
	data, err := Read(name)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func DefaultGoMod(opts Options) string {
	version := strings.TrimSpace(opts.PlumsVersion)
	if version == "" {
		version = "v0.0.0"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n\n", defaultUserModulePath)
	fmt.Fprintf(&b, "go %s\n\n", defaultGoVersion)
	fmt.Fprintf(&b, "require %s %s\n", defaultPlumsModulePath, version)
	if dir := strings.TrimSpace(opts.PlumsModuleDir); dir != "" {
		fmt.Fprintf(&b, "\nreplace %s => %s\n", defaultPlumsModulePath, filepath.ToSlash(dir))
	}
	return b.String()
}

func firstOptions(opts []Options) Options {
	if len(opts) == 0 {
		return Options{}
	}
	return opts[0]
}
