package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

type RenderConfig struct {
	Version int                   `json:"version"`
	Layouts map[string]LayoutNode `json:"layouts"`
	// Menu is the ordered list of user-selectable layout ids (the ones the
	// layout cycle and palette offer). It is the data-driven knob for adding a
	// layout: define it under "layouts" and list its key here. When empty, the
	// legacy split/zen keys are recognised for backwards compatibility.
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

	// slotID is the component's position in the layout tree (e.g. "/0/2"). It is
	// assigned during traversal, not deserialized, and identifies a stateful
	// public component instance so the same component used in two slots keeps
	// independent state.
	slotID string
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
	Background      []uint8 `json:"background"`
	Foreground      []uint8 `json:"foreground"`
	Muted           []uint8 `json:"muted"`
	Accent          []uint8 `json:"accent"`
	BackgroundToken string  `json:"background_token"`
	ForegroundToken string  `json:"foreground_token"`
	MutedToken      string  `json:"muted_token"`
	AccentToken     string  `json:"accent_token"`
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

// NewRenderConfig returns an internal render-config scaffold: an empty layout
// set plus the fixed overlay definitions (the slash-command dropdown and command
// palette popup). Layouts and the menu are populated at startup by
// InstallPublicLayout from the registered layout plugins — there is no
// user-authored layout data file. The overlays are app-internal chrome, not a
// user-authored layout, so they live here as Go values rather than a file.
func NewRenderConfig() *RenderConfig {
	return &RenderConfig{
		Version:  1,
		Layouts:  map[string]LayoutNode{},
		Overlays: defaultOverlays(),
	}
}

func defaultOverlays() map[string]OverlayNode {
	return map[string]OverlayNode{
		"slash_command_dropdown": {
			EnabledWhen: "!state.PopupOpen && len(state.SlashCommands()) > 0",
			Width: OverlayWidthNode{
				Preferred: 44,
				Min:       20,
				Max:       json.RawMessage(`"state.width - 2"`),
			},
			Style: StyleNode{
				BackgroundToken: "bg_surface",
				ForegroundToken: "text",
				MutedToken:      "text_muted",
				AccentToken:     "accent",
			},
		},
		"command_palette_popup": {
			EnabledWhen: `state.PopupOpen && (state.EffectiveLayout() != "split" || state.width < MinSplitLayoutWidth)`,
		},
	}
}

func (cfg *RenderConfig) AvailableLayoutTypes() []LayoutType {
	if cfg == nil {
		return nil
	}
	// Explicit menu wins: a fully data-driven, ordered selection. Each entry
	// must resolve to a defined layout.
	if len(cfg.Menu) > 0 {
		layouts := make([]LayoutType, 0, len(cfg.Menu))
		for _, name := range cfg.Menu {
			if name == "" {
				continue
			}
			if _, ok := cfg.Layouts[name]; ok {
				layouts = append(layouts, LayoutType(name))
			}
		}
		return layouts
	}

	// Legacy fallback (no menu declared): recognise historical keys in their
	// old order.
	layouts := make([]LayoutType, 0, 2)
	if _, ok := cfg.Layouts["split"]; ok {
		layouts = append(layouts, LayoutSplit)
	}
	if _, ok := cfg.Layouts["zen"]; ok {
		layouts = append(layouts, LayoutZen)
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
	if !ok {
		return nil, fmt.Errorf("layout %q not found", name)
	}
	if node.MinWidth == "MinSplitLayoutWidth" && state.width < MinSplitLayoutWidth && node.Fallback != "" {
		return buildLayout(state, cfg, node.Fallback)
	}
	node.slotID = ""
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
		replacement.slotID = node.slotID
		node = replacement
	}

	if node.Type == "div" || len(node.Children) > 0 {
		div := components.NewDiv()
		applyNodeProperties(state, div, node)
		for i, childNode := range node.Children {
			childNode.slotID = node.slotID + "/" + strconv.Itoa(i)
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
	return buildRegisteredComponent(state, node)
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
	} else if color, ok := themeColor(node.BackgroundToken); ok {
		style.SetBackground(color.R, color.G, color.B)
	}
	if len(node.Foreground) == 3 {
		style.SetForeground(node.Foreground[0], node.Foreground[1], node.Foreground[2])
	} else if color, ok := themeColor(node.ForegroundToken); ok {
		style.SetForeground(color.R, color.G, color.B)
	}
	return style
}

func themeColor(name string) (theme.Color, bool) {
	switch name {
	case "bg_backdrop":
		return theme.BgBackdrop, true
	case "bg_input":
		return theme.BgInput, true
	case "bg_base":
		return theme.BgBase, true
	case "bg_surface":
		return theme.BgSurface, true
	case "bg_panel":
		return theme.BgPanel, true
	case "bg_raised":
		return theme.BgRaised, true
	case "bg_highlight":
		return theme.BgHighlight, true
	case "bg_selected":
		return theme.BgSelected, true
	case "text_bright":
		return theme.TextBright, true
	case "text":
		return theme.Text, true
	case "text_soft":
		return theme.TextSoft, true
	case "text_muted":
		return theme.TextMuted, true
	case "text_faint":
		return theme.TextFaint, true
	case "text_dim":
		return theme.TextDim, true
	case "accent":
		return theme.Accent, true
	case "accent_bold":
		return theme.AccentBold, true
	case "accent_soft":
		return theme.AccentSoft, true
	case "border_accent":
		return theme.BorderAccent, true
	case "border":
		return theme.Border, true
	default:
		return theme.Color{}, false
	}
}

func hasContainerProperties(node LayoutNode) bool {
	return len(node.Size.Width) > 0 || len(node.Size.Height) > 0 || !isEmptyPadding(node.Padding) || !isEmptyStyle(node.Style)
}

func isEmptyPadding(node PaddingNode) bool {
	return node.Top == nil && node.Right == nil && node.Bottom == nil && node.Left == nil
}

func isEmptyStyle(node StyleNode) bool {
	return len(node.Background) == 0 &&
		len(node.Foreground) == 0 &&
		len(node.Muted) == 0 &&
		len(node.Accent) == 0 &&
		node.BackgroundToken == "" &&
		node.ForegroundToken == "" &&
		node.MutedToken == "" &&
		node.AccentToken == ""
}
