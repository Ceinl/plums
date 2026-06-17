package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Ceinl/plums/internal/app/defaults"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
)

type RenderConfig struct {
	Version int                   `json:"version"`
	Layouts map[string]LayoutNode `json:"layouts"`
	// Menu is the ordered list of user-selectable layout ids (the ones the
	// layout cycle and palette offer). It is the data-driven knob for adding a
	// layout: define it under "layouts" and list its key here. When empty, the
	// built-in chat/split/fullscreen set is recognised for backwards
	// compatibility.
	Menu     []string               `json:"menu"`
	Overlays map[string]OverlayNode `json:"overlays"`
}

type LayoutNode struct {
	Type          string            `json:"type"`
	Component     string            `json:"component"`
	Size          SizeNode          `json:"size"`
	Direction     string            `json:"direction"`
	AlignItems    string            `json:"align_items"`
	MinWidth      string            `json:"min_width"`
	Fallback      string            `json:"fallback"`
	Padding       PaddingNode       `json:"padding"`
	Style         StyleNode         `json:"style"`
	Children      []LayoutNode      `json:"children"`
	WhenPopupOpen *LayoutNode       `json:"when_popup_open"`
	Variants      map[string]string `json:"variants"`
}

type SizeNode struct {
	Width  json.RawMessage `json:"width"`
	Height json.RawMessage `json:"height"`
}

type PaddingNode struct {
	Top    *float64 `json:"top"`
	Right  *float64 `json:"right"`
	Bottom *float64 `json:"bottom"`
	Left   *float64 `json:"left"`
}

type StyleNode struct {
	Background []uint8 `json:"background"`
	Foreground []uint8 `json:"foreground"`
	Muted      []uint8 `json:"muted"`
	Accent     []uint8 `json:"accent"`
}

type OverlayNode struct {
	EnabledWhen string           `json:"enabled_when"`
	Width       OverlayWidthNode `json:"width"`
	Style       StyleNode        `json:"style"`
}

type OverlayWidthNode struct {
	Preferred int             `json:"preferred"`
	Min       int             `json:"min"`
	Max       json.RawMessage `json:"max"`
}

func LoadRenderConfig(path string) (*RenderConfig, error) {
	// The single source of truth for the built-in layout is the embedded
	// defaults/layout.json — the same bytes seeded to disk — so there is no
	// hand-maintained second copy to drift out of sync.
	data, err := defaults.Read("layout.json")
	if err != nil {
		return nil, err
	}
	if path != "" {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}

	var cfg RenderConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported layout config version %d", cfg.Version)
	}
	if len(cfg.Layouts) == 0 {
		return nil, errors.New("layout config has no layouts")
	}
	return &cfg, nil
}

func (cfg *RenderConfig) AvailableLayoutTypes() []LayoutType {
	if cfg == nil {
		return nil
	}
	// Explicit menu wins: a fully data-driven, ordered selection. Each entry
	// must resolve to a defined layout ("chat" also accepts a "default" node).
	if len(cfg.Menu) > 0 {
		layouts := make([]LayoutType, 0, len(cfg.Menu))
		for _, name := range cfg.Menu {
			if name == "" {
				continue
			}
			if _, ok := cfg.Layouts[name]; ok {
				layouts = append(layouts, LayoutType(name))
			} else if name == "chat" {
				if _, ok := cfg.Layouts["default"]; ok {
					layouts = append(layouts, LayoutChat)
				}
			}
		}
		return layouts
	}

	// Legacy fallback (no menu declared): recognise the built-in keys in their
	// historical order.
	layouts := make([]LayoutType, 0, 4)
	if _, ok := cfg.Layouts["chat"]; ok {
		layouts = append(layouts, LayoutChat)
	} else if _, ok := cfg.Layouts["default"]; ok {
		layouts = append(layouts, LayoutChat)
	}
	if _, ok := cfg.Layouts["split"]; ok {
		layouts = append(layouts, LayoutSplit)
	}
	if _, ok := cfg.Layouts["zen"]; ok {
		layouts = append(layouts, LayoutZen)
	}
	if _, ok := cfg.Layouts["fullscreen"]; ok {
		layouts = append(layouts, LayoutFullscreen)
	}
	return layouts
}

func overlayEnabled(state *State, cfg *RenderConfig, name string) bool {
	if cfg == nil || cfg.Overlays == nil {
		return false
	}
	overlay, ok := cfg.Overlays[name]
	if !ok {
		return false
	}
	switch name {
	case "slash_command_dropdown":
		return !state.PopupOpen && state.EditorDropdownOpen()
	case "command_palette_popup":
		return state.PopupOpen && (state.EffectiveLayout() != LayoutSplit || state.width < MinSplitLayoutWidth)
	default:
		return overlay.EnabledWhen == ""
	}
}

func resolveOverlayMax(state *State, raw json.RawMessage, fallback int) int {
	if len(raw) == 0 {
		return fallback
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s == "state.width - 2" {
		return state.width - 2
	}
	return fallback
}

func buildLayout(state *State, cfg *RenderConfig, name string) (layout.Component, error) {
	node, ok := cfg.Layouts[name]
	if !ok && name == "chat" {
		node, ok = cfg.Layouts["default"]
	}
	if !ok && name == "chat" {
		node, ok = cfg.Layouts["fullscreen"]
	}
	if !ok {
		return nil, fmt.Errorf("layout %q not found", name)
	}
	if node.MinWidth == "MinSplitLayoutWidth" && state.width < MinSplitLayoutWidth && node.Fallback != "" {
		return buildLayout(state, cfg, node.Fallback)
	}
	return buildNode(state, node)
}

func buildNode(state *State, node LayoutNode) (layout.Component, error) {
	if state.PopupOpen && node.WhenPopupOpen != nil {
		replacement := *node.WhenPopupOpen
		replacement.Size = node.Size
		if isEmptyPadding(replacement.Padding) {
			replacement.Padding = node.Padding
		}
		if isEmptyStyle(replacement.Style) {
			replacement.Style = node.Style
		}
		node = replacement
	}

	if node.Type == "div" || len(node.Children) > 0 {
		div := components.NewDiv()
		applyNodeProperties(state, div, node)
		for _, childNode := range node.Children {
			child, err := buildNode(state, childNode)
			if err != nil {
				return nil, err
			}
			div.AppendChild(child)
		}
		return div, nil
	}

	component, err := buildComponent(state, node)
	if err != nil {
		return nil, err
	}
	if hasContainerProperties(node) {
		div := components.NewDiv()
		applyNodeProperties(state, div, node)
		div.AppendChild(component)
		return div, nil
	}
	return component, nil
}

func buildComponent(state *State, node LayoutNode) (layout.Component, error) {
	switch node.Component {
	case "chat_output":
		return newChatLog(state), nil
	case "status_separator", "vertical_status_separator":
		sep := components.NewSeparator()
		sep.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
		return sep, nil
	case "editor", "editor_or_palette":
		return state.Editor, nil
	case "input_box", "text_box":
		box := components.NewInputBox(state.Editor)
		box.SetStatusSegments(chatStatusSegments(state))
		return box, nil
	case "command_palette_panel":
		popup := components.NewPopup()
		popup.SetPanel(true)
		popup.SetTitle(state.PaletteTitle())
		popup.SetQuery(state.PaletteSearch())
		popup.SetItems(state.PaletteItems(), state.PaletteIndex)
		return popup, nil
	case "info_tabs":
		tabs := components.NewInfoTabs()
		tabs.SetTabs([]components.InfoTab{
			{Label: "AI output", Active: state.InfoView == InfoViewAI},
			{Label: "Git diff", Active: state.InfoView == InfoViewGitDiff},
		})
		return tabs, nil
	case "sessions", "sessions_vertical":
		return newSessions(state, components.SessionsVertical), nil
	case "sessions_horizontal":
		return newSessions(state, components.SessionsHorizontal), nil
	case "info_view":
		if state.InfoView == InfoViewGitDiff && node.Variants["git_diff"] == "git_diff_log" {
			return newGitDiffLog(state), nil
		}
		return newChatLog(state), nil
	case "fullscreen_view":
		return newFullscreenView(state), nil
	case "split_status_bar":
		bar := components.NewStatusBar()
		bar.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
		bar.SetSession(state.SessionTitle)
		bar.SetMode(state.Mode)
		bar.SetModel(state.ModelProvider, state.ModelID)
		return bar, nil
	case "status_bar":
		bar := components.NewStatusBar()
		bar.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
		bar.SetSession(state.SessionTitle)
		bar.SetMode(state.Mode)
		bar.SetModel(state.ModelProvider, state.ModelID)
		bar.SetShowSession(false)
		return bar, nil
	default:
		return nil, fmt.Errorf("unknown component %q", node.Component)
	}
}

func applyNodeProperties(state *State, div *components.Div, node LayoutNode) {
	div.SetSize(resolveUnit(state, node.Size.Width, layout.Unit{Type: layout.UnitGrow, Value: 1}), resolveUnit(state, node.Size.Height, layout.Unit{Type: layout.UnitGrow, Value: 1}))
	if node.Direction == "row" {
		div.SetDirection(layout.Row)
	} else {
		div.SetDirection(layout.Column)
	}
	switch node.AlignItems {
	case "center":
		div.AlignItems(layout.ACenter)
	case "right":
		div.AlignItems(layout.ARight)
	}
	div.SetPadding(resolvePadding(node.Padding))
	if !isEmptyStyle(node.Style) {
		div.SetStyle(resolveStyle(node.Style))
	}
}

func resolveUnit(state *State, raw json.RawMessage, fallback layout.Unit) layout.Unit {
	if len(raw) == 0 {
		return fallback
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return layout.Unit{Type: layout.UnitPx, Value: n}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fallback
	}
	if s == "grow" {
		return layout.Unit{Type: layout.UnitGrow, Value: 1}
	}
	if s == "state.SplitLeftPercent%" {
		return layout.Unit{Type: layout.UnitPercent, Value: float64(state.SplitLeftPercent())}
	}
	if strings.HasSuffix(s, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err == nil {
			return layout.Unit{Type: layout.UnitPercent, Value: v}
		}
	}
	return fallback
}

func resolvePadding(node PaddingNode) layout.Padding {
	return layout.Padding{
		Top:    paddingUnit(node.Top),
		Right:  paddingUnit(node.Right),
		Bottom: paddingUnit(node.Bottom),
		Left:   paddingUnit(node.Left),
	}
}

func paddingUnit(v *float64) layout.Unit {
	if v == nil {
		return layout.Unit{}
	}
	return layout.Unit{Type: layout.UnitPx, Value: *v}
}

func resolveStyle(node StyleNode) layout.Style {
	style := layout.Style{}
	if len(node.Background) == 3 {
		style.SetBackground(node.Background[0], node.Background[1], node.Background[2])
	}
	if len(node.Foreground) == 3 {
		style.SetForeground(node.Foreground[0], node.Foreground[1], node.Foreground[2])
	}
	return style
}

func hasContainerProperties(node LayoutNode) bool {
	return len(node.Size.Width) > 0 || len(node.Size.Height) > 0 || !isEmptyPadding(node.Padding) || !isEmptyStyle(node.Style)
}

func isEmptyPadding(node PaddingNode) bool {
	return node.Top == nil && node.Right == nil && node.Bottom == nil && node.Left == nil
}

func isEmptyStyle(node StyleNode) bool {
	return len(node.Background) == 0 && len(node.Foreground) == 0 && len(node.Muted) == 0 && len(node.Accent) == 0
}
