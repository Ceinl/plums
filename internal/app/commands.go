package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Ceinl/plums/internal/app/defaults"
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

func LoadCommandConfig(path string) (*CommandConfig, error) {
	// Single source of truth: the embedded defaults/commands.json, the same
	// bytes seeded to disk — no hand-maintained second copy to drift.
	data, err := defaults.Read("commands.json")
	if err != nil {
		return nil, err
	}
	if path != "" {
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
	case "cycle_toolcall_visibility":
		return PaletteActionCycleToolCallVisibility
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
		if state.EffectiveLayout() != LayoutSplit && item.Action == "adjust_output_percentage" {
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
	value = strings.ReplaceAll(value, "{toolcall_visibility}", s.ToolCallVisibilityLabel())
	value = strings.ReplaceAll(value, "{layout}", s.LayoutLabel())
	value = strings.ReplaceAll(value, "{output_percent}", strconv.Itoa(s.SplitOutputPercent()))
	value = strings.ReplaceAll(value, "{backend_provider}", s.BackendProvider)
	return value
}
