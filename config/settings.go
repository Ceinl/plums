package config

import "github.com/Ceinl/plums/capabilities"

// SettingsDefaults supplies concrete fallbacks for tri-state Opts fields whose
// "unset" must resolve to a meaningful value when projecting to
// capabilities.Settings. These mirror today's hardcoded runtime defaults.
type SettingsDefaults struct {
	HideThinking   bool
	SplitLeftWidth int
	OutputPercent  int
	ClearHistory   bool
}

// ToSettings projects resolved Opts into the kernel/runtime-facing
// capabilities.Settings. Tri-state fields resolve through def; Pref fields use
// their pinned value (Dynamic and unset both resolve to "" for now — Phase 5
// wires the dynamic-prefs store).
func (o Opts) ToSettings(def SettingsDefaults) capabilities.Settings {
	return capabilities.Settings{
		Backend:          o.Backend.Value(),
		DefaultLayout:    o.DefaultLayout.Value(),
		Mode:             o.Mode.Value(),
		Theme:            o.Theme,
		Keybinds:         o.Keybinds,
		ClipboardCommand: o.ClipboardCommand,
		HideThinking:     o.HideThinking.Value(def.HideThinking),
		SplitLeftWidth:   o.SplitLeftWidth.Value(def.SplitLeftWidth),
		OutputPercent:    o.OutputPercent.Value(def.OutputPercent),
		ClearHistory:     o.ClearHistory.Value(def.ClearHistory),
		Disable:          o.Disable,
	}
}
