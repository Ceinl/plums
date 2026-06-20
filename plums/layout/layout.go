package layout

import (
	"encoding/json"
	"fmt"

	"github.com/Ceinl/plums/capabilities"
)

type Layout struct {
	name string
	root capabilities.Node
}

func Named(name string, root capabilities.Node) capabilities.Layout {
	return Layout{name: name, root: root}
}

func Split(left, right capabilities.Node) capabilities.Layout {
	return Named("split", Row(left, right))
}

func (l Layout) Name() string {
	return l.name
}

func (l Layout) Tree() capabilities.Node {
	return l.root
}

type hiddenLayout struct {
	capabilities.Layout
}

func Hidden(layout capabilities.Layout) capabilities.Layout {
	return hiddenLayout{Layout: layout}
}

func (l hiddenLayout) Selectable() bool {
	return false
}

type Node struct {
	Kind              string
	ComponentName     string
	DirectionName     string
	AlignItemsName    string
	MinWidthValue     string
	FallbackName      string
	WidthValue        any
	HeightValue       any
	PaddingValue      Padding
	StyleValue        Style
	ChildrenNodes     []capabilities.Node
	WhenPopupOpenNode capabilities.Node
	VariantsMap       map[string]string
}

type Padding struct {
	Top    *float64
	Right  *float64
	Bottom *float64
	Left   *float64
}

type Style struct {
	Background      []uint8
	Foreground      []uint8
	Muted           []uint8
	Accent          []uint8
	BackgroundToken string
	ForegroundToken string
	MutedToken      string
	AccentToken     string
}

const (
	ColorBgBackdrop  = "bg_backdrop"
	ColorBgInput     = "bg_input"
	ColorBgBase      = "bg_base"
	ColorBgSurface   = "bg_surface"
	ColorBgPanel     = "bg_panel"
	ColorBgRaised    = "bg_raised"
	ColorBgHighlight = "bg_highlight"
	ColorBgSelected  = "bg_selected"

	ColorTextBright = "text_bright"
	ColorText       = "text"
	ColorTextSoft   = "text_soft"
	ColorTextMuted  = "text_muted"
	ColorTextFaint  = "text_faint"
	ColorTextDim    = "text_dim"

	ColorAccent       = "accent"
	ColorAccentBold   = "accent_bold"
	ColorAccentSoft   = "accent_soft"
	ColorBorderAccent = "border_accent"
	ColorBorder       = "border"
)

func ThemeBg(background string) Style {
	return Style{BackgroundToken: background}
}

func ThemeStyle(background, foreground string) Style {
	return Style{BackgroundToken: background, ForegroundToken: foreground}
}

func Row(children ...capabilities.Node) *Node {
	return &Node{Kind: "div", DirectionName: "row", ChildrenNodes: children}
}

func Column(children ...capabilities.Node) *Node {
	return &Node{Kind: "div", DirectionName: "column", ChildrenNodes: children}
}

func Component(name string) *Node {
	return &Node{ComponentName: name}
}

func Editor() *Node {
	return Component("editor")
}

func EditorOrPalette() *Node {
	return Component("editor_or_palette")
}

func InputBox() *Node {
	return Component("input_box")
}

func Chat() *Node {
	return Component("chat_output")
}

func InfoView() *Node {
	return Component("info_view")
}

func Tabs() *Node {
	return Component("info_tabs")
}

func StatusBar() *Node {
	return Component("status_bar")
}

func SplitStatusBar() *Node {
	return Component("split_status_bar")
}

func Sessions() *Node {
	return Component("sessions")
}

func VerticalSeparator() *Node {
	return Component("vertical_status_separator")
}

func Separator() *Node {
	return Component("status_separator")
}

func (n *Node) Width(value any) *Node {
	n.WidthValue = value
	return n
}

func (n *Node) Height(value any) *Node {
	n.HeightValue = value
	return n
}

func (n *Node) MinWidth(value string) *Node {
	n.MinWidthValue = value
	return n
}

func (n *Node) Fallback(value string) *Node {
	n.FallbackName = value
	return n
}

func (n *Node) AlignItems(value string) *Node {
	n.AlignItemsName = value
	return n
}

func (n *Node) Padding(value Padding) *Node {
	n.PaddingValue = value
	return n
}

func (n *Node) Style(value Style) *Node {
	n.StyleValue = value
	return n
}

func (n *Node) WhenPopupOpen(value capabilities.Node) *Node {
	n.WhenPopupOpenNode = value
	return n
}

func (n *Node) Variants(value map[string]string) *Node {
	n.VariantsMap = cloneVariants(value)
	return n
}

func FromJSON(name string, data []byte) (capabilities.Layout, error) {
	var cfg jsonLayoutConfig
	if err := json.Unmarshal(data, &cfg); err == nil && len(cfg.Layouts) > 0 {
		layoutName := name
		node, ok := cfg.Layouts[layoutName]
		if !ok && layoutName == "" && len(cfg.Layouts) == 1 {
			for key, value := range cfg.Layouts {
				layoutName = key
				node = value
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("layout %q not found in JSON", name)
		}
		return Named(layoutName, node.toPublic()), nil
	}

	var node jsonNode
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("layout name is required")
	}
	return Named(name, node.toPublic()), nil
}

type jsonLayoutConfig struct {
	Layouts map[string]jsonNode `json:"layouts"`
}

type jsonNode struct {
	Type          string            `json:"type"`
	Component     string            `json:"component"`
	Size          jsonSize          `json:"size"`
	Direction     string            `json:"direction"`
	AlignItems    string            `json:"align_items"`
	MinWidth      string            `json:"min_width"`
	Fallback      string            `json:"fallback"`
	Padding       jsonPadding       `json:"padding"`
	Style         jsonStyle         `json:"style"`
	Children      []jsonNode        `json:"children"`
	WhenPopupOpen *jsonNode         `json:"when_popup_open"`
	Variants      map[string]string `json:"variants"`
}

type jsonSize struct {
	Width  json.RawMessage `json:"width"`
	Height json.RawMessage `json:"height"`
}

type jsonPadding struct {
	Top    *float64 `json:"top"`
	Right  *float64 `json:"right"`
	Bottom *float64 `json:"bottom"`
	Left   *float64 `json:"left"`
}

type jsonStyle struct {
	Background      []uint8 `json:"background"`
	Foreground      []uint8 `json:"foreground"`
	Muted           []uint8 `json:"muted"`
	Accent          []uint8 `json:"accent"`
	BackgroundToken string  `json:"background_token"`
	ForegroundToken string  `json:"foreground_token"`
	MutedToken      string  `json:"muted_token"`
	AccentToken     string  `json:"accent_token"`
}

func (n jsonNode) toPublic() *Node {
	out := &Node{
		Kind:           n.Type,
		ComponentName:  n.Component,
		DirectionName:  n.Direction,
		AlignItemsName: n.AlignItems,
		MinWidthValue:  n.MinWidth,
		FallbackName:   n.Fallback,
		WidthValue:     cloneRaw(n.Size.Width),
		HeightValue:    cloneRaw(n.Size.Height),
		PaddingValue: Padding{
			Top:    n.Padding.Top,
			Right:  n.Padding.Right,
			Bottom: n.Padding.Bottom,
			Left:   n.Padding.Left,
		},
		StyleValue: Style{
			Background:      cloneBytes(n.Style.Background),
			Foreground:      cloneBytes(n.Style.Foreground),
			Muted:           cloneBytes(n.Style.Muted),
			Accent:          cloneBytes(n.Style.Accent),
			BackgroundToken: n.Style.BackgroundToken,
			ForegroundToken: n.Style.ForegroundToken,
			MutedToken:      n.Style.MutedToken,
			AccentToken:     n.Style.AccentToken,
		},
		VariantsMap: cloneVariants(n.Variants),
	}
	for _, child := range n.Children {
		out.ChildrenNodes = append(out.ChildrenNodes, child.toPublic())
	}
	if n.WhenPopupOpen != nil {
		out.WhenPopupOpenNode = n.WhenPopupOpen.toPublic()
	}
	return out
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneBytes(value []uint8) []uint8 {
	if len(value) == 0 {
		return nil
	}
	return append([]uint8(nil), value...)
}

func cloneVariants(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]string, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}
