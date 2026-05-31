package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CommandConfig struct {
	Version       int                   `json:"version"`
	SlashCommands []SlashCommandConfig  `json:"slash_commands"`
	Palette       PaletteConfig         `json:"palette"`
	Actions       map[string]ActionSpec `json:"actions"`
}

type SlashCommandConfig struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
	Action string `json:"action"`
}

type PaletteConfig struct {
	Title               string              `json:"title"`
	EmptySessionsTitle  string              `json:"empty_sessions_title"`
	EmptySessionsDetail string              `json:"empty_sessions_detail"`
	Items               []PaletteItemConfig `json:"items"`
}

type PaletteItemConfig struct {
	Title     string            `json:"title"`
	Detail    string            `json:"detail"`
	Action    string            `json:"action"`
	Disabled  bool              `json:"disabled"`
	TitleWhen map[string]string `json:"title_when"`
}

type ActionSpec struct {
	Kind          string         `json:"kind"`
	StatusMessage string         `json:"status_message"`
	Adjustment    AdjustmentSpec `json:"adjustment"`
}

type AdjustmentSpec struct {
	Min  int `json:"min"`
	Max  int `json:"max"`
	Step int `json:"step"`
}

const defaultCommandsJSON = `{
  "version": 1,
  "slash_commands": [
    { "name": "/new", "detail": "Create a fresh opencode session", "action": "new_session" },
    { "name": "/command", "detail": "Open the command palette", "action": "open_palette" },
    { "name": "/backend", "detail": "Switch backend provider", "action": "backend_list" },
    { "name": "/skills", "detail": "Load an opencode skill", "action": "skills_list" },
    { "name": "/sessions", "detail": "Open existing opencode sessions", "action": "sessions_list" }
  ],
  "palette": {
    "title": "Command Palette",
    "empty_sessions_title": "No sessions",
    "empty_sessions_detail": "No opencode sessions found",
    "items": [
      { "title": "Change model", "detail": "Select model for future prompts", "action": "change_model" },
      { "title": "Backend provider", "detail": "Current backend: {backend_provider}", "action": "backend_list" },
      { "title": "Start new session", "detail": "Create a fresh opencode session", "action": "new_session" },
      {
        "title": "Switch mode",
        "detail": "Current mode: {mode}",
        "action": "switch_mode",
        "title_when": { "mode=plan": "Switch to build mode", "mode=build": "Switch to plan mode" }
      },
      { "title": "Layouts", "detail": "Select layout - current: {layout}", "action": "switch_layout" },
      { "title": "Thinking visibility", "detail": "Current: {thinking_visibility} (cycles full/title/hidden)", "action": "cycle_thinking_visibility" },
      { "title": "Output percentage", "detail": "Left/Right adjust - current: {output_percent}%", "action": "adjust_output_percentage" },
      { "title": "Skills list", "detail": "Load an opencode skill for the next prompt", "action": "skills_list" },
      { "title": "Sessions list", "detail": "Open existing opencode sessions", "action": "sessions_list" }
    ]
  },
  "actions": {
    "change_model": { "kind": "builtin" },
    "backend_list": { "kind": "builtin" },
    "new_session": { "kind": "builtin" },
    "open_palette": { "kind": "builtin" },
    "switch_mode": { "kind": "builtin" },
    "switch_layout": { "kind": "builtin" },
    "cycle_thinking_visibility": { "kind": "builtin" },
    "adjust_output_percentage": { "kind": "builtin", "adjustment": { "min": 25, "max": 75, "step": 5 } },
    "skills_list": { "kind": "builtin" },
    "sessions_list": { "kind": "builtin" }
  }
}`

func LoadCommandConfig(path string) (*CommandConfig, error) {
	data := []byte(defaultCommandsJSON)
	if path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}

	var cfg CommandConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func DefaultCommandConfig() *CommandConfig {
	cfg, err := LoadCommandConfig("")
	if err != nil {
		panic(err)
	}
	return cfg
}

func (cfg *CommandConfig) validate() error {
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported command config version %d", cfg.Version)
	}
	if len(cfg.SlashCommands) == 0 {
		return errors.New("command config has no slash commands")
	}
	if len(cfg.Palette.Items) == 0 {
		return errors.New("command config has no palette items")
	}
	if cfg.Palette.Title == "" {
		cfg.Palette.Title = "Command Palette"
	}
	if cfg.Palette.EmptySessionsTitle == "" {
		cfg.Palette.EmptySessionsTitle = "No sessions"
	}
	if cfg.Palette.EmptySessionsDetail == "" {
		cfg.Palette.EmptySessionsDetail = "No opencode sessions found"
	}
	for _, command := range cfg.SlashCommands {
		if command.Name == "" || command.Action == "" {
			return errors.New("slash commands require name and action")
		}
		if _, ok := cfg.Actions[command.Action]; !ok {
			return fmt.Errorf("slash command %q references unknown action %q", command.Name, command.Action)
		}
	}
	for _, item := range cfg.Palette.Items {
		if item.Title == "" || item.Action == "" {
			return errors.New("palette items require title and action")
		}
		if _, ok := cfg.Actions[item.Action]; !ok {
			return fmt.Errorf("palette item %q references unknown action %q", item.Title, item.Action)
		}
	}
	return nil
}

func (cfg *CommandConfig) actionFor(name string) PaletteAction {
	if spec, ok := cfg.Actions[name]; ok && spec.Kind == "builtin" {
		return builtinAction(name)
	}
	return PaletteActionNone
}

func builtinAction(name string) PaletteAction {
	switch name {
	case "change_model":
		return PaletteActionChangeModel
	case "backend_list":
		return PaletteActionBackendList
	case "open_palette":
		return PaletteActionOpenPalette
	case "new_session":
		return PaletteActionNewSession
	case "switch_mode":
		return PaletteActionSwitchMode
	case "switch_layout":
		return PaletteActionLayoutsList
	case "cycle_thinking_visibility":
		return PaletteActionCycleThinkingVisibility
	case "sessions_list":
		return PaletteActionSessionsList
	case "skills_list":
		return PaletteActionSkillsList
	default:
		return PaletteActionNone
	}
}

func (cfg *CommandConfig) commandItems(state *State) []paletteCommandItem {
	items := make([]paletteCommandItem, 0, len(cfg.Palette.Items))
	for _, item := range cfg.Palette.Items {
		if state.EffectiveLayout() == LayoutFullscreen && item.Action == "adjust_output_percentage" {
			continue
		}
		action := cfg.actionFor(item.Action)
		spec := cfg.Actions[item.Action]
		items = append(items, paletteCommandItem{
			Title:    state.expandCommandTemplate(item.effectiveTitle(state)),
			Detail:   state.expandCommandTemplate(item.Detail),
			Action:   action,
			Adjust:   spec.Adjustment.Step != 0,
			Min:      spec.Adjustment.Min,
			Max:      spec.Adjustment.Max,
			Step:     spec.Adjustment.Step,
			Disabled: item.Disabled,
		})
	}
	return items
}

func (item PaletteItemConfig) effectiveTitle(state *State) string {
	for condition, title := range item.TitleWhen {
		key, value, ok := strings.Cut(condition, "=")
		if ok && strings.TrimSpace(key) == "mode" && strings.TrimSpace(value) == state.Mode {
			return title
		}
	}
	return item.Title
}

func (s *State) expandCommandTemplate(value string) string {
	value = strings.ReplaceAll(value, "{mode}", s.Mode)
	value = strings.ReplaceAll(value, "{thinking_visibility}", s.ThinkingVisibilityLabel())
	value = strings.ReplaceAll(value, "{layout}", s.LayoutLabel())
	value = strings.ReplaceAll(value, "{output_percent}", strconv.Itoa(s.SplitOutputPercent()))
	value = strings.ReplaceAll(value, "{backend_provider}", s.BackendProvider)
	return value
}
