package defaults

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed config.toml layout.json commands.json
var files embed.FS

const (
	configToml   = "config.toml"
	layoutJSON   = "layout.json"
	commandsJSON = "commands.json"
)

// Read returns the content of a built-in default config file by name.
func Read(name string) ([]byte, error) {
	return files.ReadFile(name)
}

// WriteDefault writes a built-in default file into dir if it does not already
// exist. name must be one of "config.toml", "layout.json" or "commands.json".
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

// WriteAll writes all three default files into dir.
func WriteAll(dir string) error {
	for _, name := range []string{configToml, layoutJSON, commandsJSON} {
		if err := WriteDefault(dir, name); err != nil {
			return err
		}
	}
	return nil
}
