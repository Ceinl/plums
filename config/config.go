// Package config is the public authoring surface for a plums user config. An
// external ~/.config/plums/config/config.go imports this package and provides a
// func Config() config.Config that returns its Opts and the plugins it wants.
//
// config builds on github.com/Ceinl/plums/capabilities (the plugin floor).
package config

import "github.com/Ceinl/plums/capabilities"

// Config is the root returned by a user config's func Config() config.Config and
// by the built-in Default Config. Opts are the non-plugin settings; Plugins are
// the units of behavior to activate (in slice order, last-wins over defaults).
type Config struct {
	Opts    Opts
	Plugins []Plugin
}

// Plugin pairs a plugin value (Self) with its typed options (Opts). The loader
// activates Self's capabilities and calls Self.Init(host, Opts).
type Plugin struct {
	Self any
	Opts any
}

// Keybind maps a key chord to a command name.
type Keybind = capabilities.Keybind

// Theme is the appearance token set (an Opt, not a plugin).
type Theme = capabilities.Theme

// RegistryKey qualifies a registry entry for Opts.Disable.
type RegistryKey = capabilities.RegistryKey

// Opts is the long, typed list of everything that is NOT a plugin: active
// selection, appearance, input, conversation visibility, split sizing, sessions,
// registry disables, and advanced timeouts. Fields left at their zero value
// inherit the Default Config's value when merged (see Merge); use the sentinel
// tri-state / Dynamic helpers below to express "set" unambiguously for fields
// whose zero value is otherwise meaningful.
type Opts struct {
	// Active selection — typical Dynamic candidates (remembered at runtime).
	Backend       Pref // backend plugin name, or Dynamic
	Model         Pref // model id, or Dynamic
	DefaultLayout Pref // layout name, or Dynamic
	Mode          Pref // agent mode, or Dynamic

	// Appearance.
	Theme Theme

	// Input.
	Keybinds         []Keybind
	ClipboardCommand string

	// Conversation visibility.
	HideThinking       Bool // tri-state: True / False / unset
	ThinkingVisibility int
	ToolCallVisibility int

	// Split sizing.
	SplitLeftWidth Count // tri-state int: unset inherits
	OutputPercent  Count

	// Sessions.
	ClearHistory Bool

	// Registry.
	Disable []RegistryKey

	// Advanced timeouts (milliseconds; unset/0 inherits the default).
	HealthTimeoutMS        int
	QuestionReplyTimeoutMS int
	RecentModelTimeoutMS   int
	ListTimeoutMS          int
	SpinnerIntervalMS      int
}

// Pref is a string-valued option that distinguishes three states: unset (zero
// value — inherit the default), Dynamic (remembered at runtime by the
// app-managed state.toml store), and a pinned literal value. An untyped string
// constant assigns directly: Backend: "opencode".
type Pref string

// dynamicSentinel is an in-band marker chosen to never collide with a real
// value. It is unexported so the only way to request Dynamic is the Dynamic
// constant below.
const dynamicSentinel Pref = "\x00plums:dynamic\x00"

// Dynamic marks a Pref field as runtime-remembered. When no stored value exists,
// it behaves as "use the default value".
const Dynamic = dynamicSentinel

// Set reports whether the Pref was assigned (any non-zero value, including
// Dynamic).
func (p Pref) Set() bool { return p != "" }

// IsDynamic reports whether the Pref is the Dynamic sentinel.
func (p Pref) IsDynamic() bool { return p == dynamicSentinel }

// Value returns the pinned string, or "" for unset or Dynamic (both of which
// fall through to the default during settings projection).
func (p Pref) Value() string {
	if p == "" || p == dynamicSentinel {
		return ""
	}
	return string(p)
}

// Bool is a tri-state boolean: Unset (zero value — inherit the default), True,
// or False. This removes the ambiguity of a plain bool whose zero value (false)
// is indistinguishable from "not set".
type Bool int8

const (
	Unset Bool = iota // inherit the default
	True
	False
)

// Set reports whether the Bool was explicitly assigned True or False.
func (b Bool) Set() bool { return b != Unset }

// Value resolves the tri-state to a concrete bool, returning def when unset.
func (b Bool) Value(def bool) bool {
	switch b {
	case True:
		return true
	case False:
		return false
	default:
		return def
	}
}

// Count is a tri-state integer for fields whose zero value (0) is a meaningful
// setting and so cannot double as "unset". The zero Count is unset; wrap a real
// value with Int (including Int(0)). A Count can also be Dynamic (runtime-
// remembered) via DynamicCount.
type Count struct {
	set     bool
	dynamic bool
	val     int
}

// Int constructs a set Count from v (Int(0) is "explicitly zero", distinct from
// the zero-value Count which is unset).
func Int(v int) Count { return Count{set: true, val: v} }

// DynamicCount marks a Count field as runtime-remembered: it counts as set (so
// it survives Merge), but resolves to the default until the dynamic-prefs store
// supplies a remembered value.
var DynamicCount = Count{set: true, dynamic: true}

// Set reports whether the Count was assigned (including Dynamic).
func (c Count) Set() bool { return c.set }

// IsDynamic reports whether the Count is the Dynamic sentinel.
func (c Count) IsDynamic() bool { return c.dynamic }

// Value resolves the Count, returning def when unset or Dynamic.
func (c Count) Value(def int) int {
	if c.set && !c.dynamic {
		return c.val
	}
	return def
}
