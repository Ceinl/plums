package app

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// VersionMetadata holds build-time version information.
type VersionMetadata struct {
	Version   string
	Commit    string
	BuildDate string
	GoVersion string
	Modified  bool
}

// LoadVersionMetadata populates version metadata from build info and
// the supplied ldflag values.
func LoadVersionMetadata(version, commit, buildDate string) VersionMetadata {
	meta := VersionMetadata{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		GoVersion: runtime.Version(),
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if meta.Commit == "unknown" {
					meta.Commit = setting.Value
				}
			case "vcs.time":
				if meta.BuildDate == "unknown" {
					meta.BuildDate = setting.Value
				}
			case "vcs.modified":
				meta.Modified = setting.Value == "true"
			}
		}
	}

	return meta
}

// FormatVersionMetadata renders metadata as a human-readable string.
func FormatVersionMetadata(meta VersionMetadata) string {
	modified := "false"
	if meta.Modified {
		modified = "true"
	}
	return fmt.Sprintf("plums %s\ncommit: %s\nbuildDate: %s\ngo: %s\nmodified: %s\n",
		meta.Version, meta.Commit, meta.BuildDate, meta.GoVersion, modified)
}
