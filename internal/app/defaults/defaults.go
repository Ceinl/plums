package defaults

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed config.toml layout.json commands.json config.go.tmpl
var files embed.FS

const (
	configToml   = "config.toml"
	layoutJSON   = "layout.json"
	commandsJSON = "commands.json"
	configGo     = "config.go"
)

// Read returns the content of a built-in default config file by name.
func Read(name string) ([]byte, error) {
	return files.ReadFile(sourceName(name))
}

// WriteDefault writes a built-in default file into dir if it does not already
// exist. name must be one of "config.toml", "layout.json", "commands.json" or
// "config.go".
func WriteDefault(dir, name string) error {
	dst := filepath.Join(dir, name)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := Read(name)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// WriteAll writes every default config file into dir that is not already
// present, leaving existing files untouched. This makes `-init-config`
// idempotent: re-running it on an older config dir adds newly-introduced
// defaults (e.g. config.go) without clobbering edits or erroring on the files
// that already exist.
func WriteAll(dir string) error {
	for _, name := range []string{configToml, layoutJSON, commandsJSON, configGo} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := WriteDefault(dir, name); err != nil {
			return err
		}
	}
	return nil
}

func sourceName(name string) string {
	if name == configGo {
		return "config.go.tmpl"
	}
	return name
}
