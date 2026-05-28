package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"plums/internal/components"
	"plums/internal/layout"
	"plums/internal/screen"
	"strconv"
	"strings"
)

var scr *screen.Screen

type RenderConfig struct {
	Version  int                    `json:"version"`
	Layouts  map[string]LayoutNode  `json:"layouts"`
	Overlays map[string]OverlayNode `json:"overlays"`
}

type LayoutNode struct {
	Type          string            `json:"type"`
	Component     string            `json:"component"`
	Size          SizeNode          `json:"size"`
	Direction     string            `json:"direction"`
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
	data := []byte(defaultLayoutJSON)
	if path != "" {
		var err error
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

// ── Colour palette ────────────────────────────────────────────────────────────
//
// Default / fullscreen layout
//   bgOutput  – the chat / output area
//   bgEditor  – the editor area
//
// Split layout
//   bgSplitEditor – left pane (slightly lighter, warm)
//   bgSplitOutput – right pane (darker, more contrast)
//   bgSeparator   – 1-column vertical divider

// ── Factory helpers ───────────────────────────────────────────────────────────

func newOutput() *components.Div {
	outputDiv := components.NewDiv()
	outputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})
	style := layout.Style{}
	style.SetBackground(22, 20, 27)
	outputDiv.SetStyle(style)
	return outputDiv
}

func newChatLog(state *State) *components.ChatLog {
	chatLog := state.ChatLog()
	msgs := make([]components.ChatMessage, len(state.messages))
	for i, m := range state.messages {
		msgs[i] = components.ChatMessage{Role: m.Role, Content: m.Content}
	}
	chatLog.SetMessages(msgs)
	chatLog.SetAiOutput(state.aioutput)
	chatLog.SetStreaming(state.IsStreaming())
	chatLog.SetThinkingVisibility(state.ThinkingMode)
	chatLog.SetScrollOffset(state.OutputScroll())
	return chatLog
}

func newGitDiffLog(state *State) *components.DiffLog {
	diffLog := state.DiffLog()
	diffLog.SetContent(state.GitDiff)
	diffLog.SetScrollOffset(state.OutputScroll())
	return diffLog
}

func newInfoTabs(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	style := layout.Style{}
	style.SetBackground(22, 20, 27)
	div.SetStyle(style)

	tabs := components.NewInfoTabs()
	tabs.SetTabs([]components.InfoTab{
		{Label: "AI output", Active: state.InfoView == InfoViewAI},
		{Label: "Git diff", Active: state.InfoView == InfoViewGitDiff},
	})
	div.AppendChild(tabs)
	return div
}

func newInfoView(state *State) layout.Component {
	if state.InfoView == InfoViewGitDiff {
		return newGitDiffLog(state)
	}
	return newChatLog(state)
}

func newHorizontalRule(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	sep := components.NewSeparator()
	sep.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
	div.AppendChild(sep)
	return div
}

func newVerticalSeparator() *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPx, Value: 1},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	return div
}

func newSplitStatusBar(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	style := layout.Style{}
	style.SetBackground(22, 20, 27)
	style.SetForeground(100, 98, 112)
	div.SetStyle(style)

	bar := components.NewStatusBar()
	bar.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
	bar.SetSession(state.SessionTitle)
	bar.SetMode(state.Mode)
	bar.SetModel(state.ModelProvider, state.ModelID)
	div.AppendChild(bar)
	return div
}

func newTextDiv(content string, w, h layout.Unit, bgR, bgG, bgB uint8) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(bgR, bgG, bgB)
	style.SetForeground(100, 98, 112)
	div.SetStyle(style)

	text := components.NewText()
	text.SetContent(content)
	div.AppendChild(text)
	return div
}

func newEditorDiv(ed *components.Editor, w, h layout.Unit) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(32, 30, 40)
	style.SetForeground(220, 218, 230)
	div.SetStyle(style)
	div.AppendChild(ed)
	return div
}

func newPalettePanel(state *State, w, h layout.Unit) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)
	div.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})

	style := layout.Style{}
	style.SetBackground(32, 30, 40)
	style.SetForeground(220, 218, 230)
	div.SetStyle(style)

	popup := components.NewPopup()
	popup.SetPanel(true)
	popup.SetTitle(state.PaletteTitle())
	popup.SetQuery(state.PaletteSearch())
	popup.SetItems(state.PaletteItems(), state.PaletteIndex)
	div.AppendChild(popup)
	return div
}

// ── Main render entry point ───────────────────────────────────────────────────

func Render(state *State, cfg *RenderConfig) {
	if cfg == nil {
		cfg, _ = LoadRenderConfig("")
	}
	if scr == nil || scr.Width() != state.width || scr.Height() != state.height {
		scr = screen.NewScreen(state.width, state.height)
	}
	scr.Clear()

	root, err := buildLayout(state, cfg, layoutName(state.EffectiveLayout()))
	if err != nil {
		root = CreateDefaultLayout(state)
	}

	root.Layout(0, 0, state.width, state.height)
	root.Render(scr)
	renderEditorDropdown(state, cfg)
	if overlayEnabled(state, cfg, "command_palette_popup") {
		popup := components.NewPopup()
		popup.SetTitle(state.PaletteTitle())
		popup.SetQuery(state.PaletteSearch())
		popup.SetItems(state.PaletteItems(), state.PaletteIndex)
		popup.Layout(0, 0, state.width, state.height)
		popup.Render(scr)
	}

	scr.Flush()
	if state.PopupOpen {
		return
	}
	cx, cy := state.Editor.CursorScreenPos()
	scr.SetCursor(cx, cy)
	scr.ShowCursor()
}

func layoutName(t LayoutType) string {
	switch t {
	case LayoutDefault:
		return "default"
	case LayoutSplit:
		return "split"
	case LayoutFullscreen:
		return "fullscreen"
	default:
		return "default"
	}
}

func (cfg *RenderConfig) AvailableLayoutTypes() []LayoutType {
	if cfg == nil {
		return nil
	}
	layouts := make([]LayoutType, 0, 3)
	if _, ok := cfg.Layouts["default"]; ok {
		layouts = append(layouts, LayoutDefault)
	}
	if _, ok := cfg.Layouts["split"]; ok {
		layouts = append(layouts, LayoutSplit)
	}
	if _, ok := cfg.Layouts["fullscreen"]; ok {
		layouts = append(layouts, LayoutFullscreen)
	}
	return layouts
}

func renderEditorDropdown(state *State, cfg *RenderConfig) {
	if !overlayEnabled(state, cfg, "slash_command_dropdown") {
		return
	}
	if suggestions := state.SkillSuggestions(); len(suggestions) > 0 {
		renderSkillDropdown(state, cfg, suggestions)
		return
	}
	renderSlashCommandDropdown(state, cfg)
}

func renderSlashCommandDropdown(state *State, cfg *RenderConfig) {
	commands := state.SlashCommands()
	if len(commands) == 0 {
		return
	}

	cx, cy := state.Editor.CursorScreenPos()
	overlay := cfg.Overlays["slash_command_dropdown"]
	w := overlay.Width.Preferred
	if w == 0 {
		w = 44
	}
	maxW := resolveOverlayMax(state, overlay.Width.Max, state.width-2)
	if w > maxW {
		w = maxW
	}
	minW := overlay.Width.Min
	if minW == 0 {
		minW = 20
	}
	if w < minW {
		return
	}
	x := cx
	if x+w > state.width {
		x = state.width - w
	}
	if x < 0 {
		x = 0
	}
	y := cy + 1
	if y+len(commands)+1 > state.height {
		y = cy - len(commands) - 1
	}
	if y < 0 {
		return
	}

	bg := ansiBgColor(overlay.Style.Background, 30, 27, 38)
	fg := ansiFgColor(overlay.Style.Foreground, 232, 229, 241)
	muted := ansiFgColor(overlay.Style.Muted, 159, 153, 176)
	accent := ansiFgColor(overlay.Style.Accent, 247, 184, 90)
	for row := 0; row < len(commands)+1 && y+row < state.height; row++ {
		for col := 0; col < w; col++ {
			scr.Set(x+col, y+row, ' ', fg, bg, "")
		}
	}
	drawOverlayText(x+1, y, w-2, "slash commands", muted, bg)
	for i, command := range commands {
		row := y + i + 1
		drawOverlayText(x+1, row, w-2, command.Name, accent, bg)
		detailX := x + 2 + len(command.Name)
		if detailX < x+w-1 {
			drawOverlayText(detailX, row, x+w-detailX-1, command.Detail, muted, bg)
		}
	}
}

func renderSkillDropdown(state *State, cfg *RenderConfig, skills []SkillSuggestion) {
	cx, cy := state.Editor.CursorScreenPos()
	overlay := cfg.Overlays["slash_command_dropdown"]
	w := overlay.Width.Preferred
	if w == 0 {
		w = 44
	}
	maxW := resolveOverlayMax(state, overlay.Width.Max, state.width-2)
	if w > maxW {
		w = maxW
	}
	minW := overlay.Width.Min
	if minW == 0 {
		minW = 20
	}
	if w < minW {
		return
	}
	x := cx
	if x+w > state.width {
		x = state.width - w
	}
	if x < 0 {
		x = 0
	}
	visible := len(skills)
	if visible > 8 {
		visible = 8
	}
	y := cy + 1
	if y+visible+1 > state.height {
		y = cy - visible - 1
	}
	if y < 0 {
		return
	}

	bg := ansiBgColor(overlay.Style.Background, 30, 27, 38)
	fg := ansiFgColor(overlay.Style.Foreground, 232, 229, 241)
	muted := ansiFgColor(overlay.Style.Muted, 159, 153, 176)
	accent := ansiFgColor(overlay.Style.Accent, 247, 184, 90)
	for row := 0; row < visible+1 && y+row < state.height; row++ {
		for col := 0; col < w; col++ {
			scr.Set(x+col, y+row, ' ', fg, bg, "")
		}
	}
	drawOverlayText(x+1, y, w-2, "skills", muted, bg)
	for i := 0; i < visible; i++ {
		skill := skills[i]
		row := y + i + 1
		name := "/skill " + skill.Name
		drawOverlayText(x+1, row, w-2, name, accent, bg)
		detailX := x + 2 + len(name)
		if detailX < x+w-1 {
			drawOverlayText(detailX, row, x+w-detailX-1, skill.Description, muted, bg)
		}
	}
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
		return !state.PopupOpen && (len(state.SlashCommands()) > 0 || len(state.SkillSuggestions()) > 0)
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

func drawOverlayText(x, y, maxW int, text, fg, bg string) {
	for i, r := range text {
		if i >= maxW {
			return
		}
		scr.Set(x+i, y, r, fg, bg, "")
	}
}

func ansiFg(r, g, b uint8) string {
	style := layout.Style{}
	style.SetForeground(r, g, b)
	return style.GetForeground()
}

func ansiFgColor(c []uint8, r, g, b uint8) string {
	if len(c) == 3 {
		return ansiFg(c[0], c[1], c[2])
	}
	return ansiFg(r, g, b)
}

func ansiBgColor(c []uint8, r, g, b uint8) string {
	if len(c) == 3 {
		return ansiBg(c[0], c[1], c[2])
	}
	return ansiBg(r, g, b)
}

func ansiBg(r, g, b uint8) string {
	style := layout.Style{}
	style.SetBackground(r, g, b)
	return style.GetBackground()
}

func buildLayout(state *State, cfg *RenderConfig, name string) (layout.Component, error) {
	node, ok := cfg.Layouts[name]
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
	case "info_view":
		if state.InfoView == InfoViewGitDiff && node.Variants["git_diff"] == "git_diff_log" {
			return newGitDiffLog(state), nil
		}
		return newChatLog(state), nil
	case "split_status_bar":
		bar := components.NewStatusBar()
		bar.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
		bar.SetSession(state.SessionTitle)
		bar.SetModel(state.ModelProvider, state.ModelID)
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
		return layout.Unit{Type: layout.UnitPersent, Value: float64(state.SplitLeftPercent())}
	}
	if strings.HasSuffix(s, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err == nil {
			return layout.Unit{Type: layout.UnitPersent, Value: v}
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

// ── Layout builders ───────────────────────────────────────────────────────────

func CreateDefaultLayout(state *State) *components.Div {
	outputDiv := newOutput()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	outputDiv.AppendChild(newChatLog(state))

	sepDiv := newHorizontalRule(state)

	inputDiv := newEditorDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 5},
	)
	inputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	root.AppendChild(outputDiv)
	root.AppendChild(sepDiv)
	root.AppendChild(inputDiv)
	return root
}

func CreateSplitLayout(state *State) *components.Div {
	if state.width < MinSplitLayoutWidth {
		return CreateNarrowSplitLayout(state)
	}

	// Left pane: editor, temporarily replaced by command palette when open.
	var leftDiv *components.Div
	if state.PopupOpen {
		leftDiv = newPalettePanel(
			state,
			layout.Unit{Type: layout.UnitPersent, Value: float64(state.SplitLeftPercent())},
			layout.Unit{Type: layout.UnitPersent, Value: 100},
		)
	} else {
		leftDiv = newEditorDiv(
			state.Editor,
			layout.Unit{Type: layout.UnitPersent, Value: float64(state.SplitLeftPercent())},
			layout.Unit{Type: layout.UnitPersent, Value: 100},
		)
		leftDiv.SetPadding(layout.Padding{
			Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
			Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
			Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
			Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
		})
	}

	// 1-column accent separator
	sep := newVerticalSeparator()
	statusSep := components.NewSeparator()
	sep.AppendChild(statusSep)

	// Right pane: chat output (fills remaining space)
	rightDiv := newOutput()
	rightDiv.SetSize(
		layout.Unit{Type: layout.UnitGrow, Value: 1},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	rightDiv.AppendChild(newInfoTabs(state))
	rightDiv.AppendChild(newInfoView(state))
	rightDiv.AppendChild(newSplitStatusBar(state))

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	root.SetDirection(layout.Row)
	root.AppendChild(leftDiv)
	root.AppendChild(sep)
	root.AppendChild(rightDiv)
	return root
}

func CreateNarrowSplitLayout(state *State) *components.Div {
	outputDiv := newOutput()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 50},
	)
	outputDiv.AppendChild(newChatLog(state))

	sepDiv := newHorizontalRule(state)

	inputDiv := newEditorDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	inputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	root.AppendChild(outputDiv)
	root.AppendChild(sepDiv)
	root.AppendChild(inputDiv)
	return root
}

func CreateFullscreenLayout(state *State) *components.Div {
	div := newEditorDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	div.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})
	return div
}

const defaultLayoutJSON = `{
  "version": 1,
  "layouts": {
    "default": {
      "type": "div",
      "size": { "width": "100%", "height": "100%" },
      "direction": "column",
      "children": [
        { "component": "chat_output", "size": { "width": "100%", "height": "grow" }, "padding": { "top": 1, "right": 2, "bottom": 1, "left": 2 }, "style": { "background": [22, 20, 27] } },
        { "component": "status_separator", "size": { "width": "100%", "height": 1 } },
        { "component": "editor", "size": { "width": "100%", "height": 5 }, "padding": { "top": 1, "right": 2, "bottom": 1, "left": 2 }, "style": { "background": [32, 30, 40], "foreground": [220, 218, 230] } }
      ]
    },
    "split": {
      "type": "div",
      "size": { "width": "100%", "height": "100%" },
      "direction": "row",
      "min_width": "MinSplitLayoutWidth",
      "children": [
        { "component": "editor_or_palette", "size": { "width": "state.SplitLeftPercent%", "height": "100%" }, "padding": { "top": 1, "right": 2, "bottom": 1, "left": 2 }, "style": { "background": [32, 30, 40], "foreground": [220, 218, 230] }, "when_popup_open": { "component": "command_palette_panel", "padding": { "top": 1, "right": 2, "bottom": 1, "left": 2 } } },
        { "component": "vertical_status_separator", "size": { "width": 1, "height": "100%" } },
        { "type": "div", "size": { "width": "grow", "height": "100%" }, "direction": "column", "padding": { "top": 1, "right": 2, "bottom": 1, "left": 2 }, "style": { "background": [22, 20, 27] }, "children": [
          { "component": "info_tabs", "size": { "width": "100%", "height": 1 }, "style": { "background": [22, 20, 27] } },
          { "component": "info_view", "variants": { "ai": "chat_log", "git_diff": "git_diff_log" } },
          { "component": "split_status_bar", "size": { "width": "100%", "height": 1 }, "style": { "background": [22, 20, 27], "foreground": [100, 98, 112] } }
        ] }
      ],
      "fallback": "narrow_split"
    },
    "narrow_split": {
      "type": "div",
      "size": { "width": "100%", "height": "100%" },
      "direction": "column",
      "children": [
        { "component": "chat_output", "size": { "width": "100%", "height": "50%" }, "padding": { "top": 1, "right": 2, "bottom": 1, "left": 2 }, "style": { "background": [22, 20, 27] } },
        { "component": "status_separator", "size": { "width": "100%", "height": 1 } },
        { "component": "editor", "size": { "width": "100%", "height": "grow" }, "padding": { "top": 1, "right": 2, "bottom": 1, "left": 2 }, "style": { "background": [32, 30, 40], "foreground": [220, 218, 230] } }
      ]
    },
    "fullscreen": { "component": "editor", "size": { "width": "100%", "height": "100%" }, "padding": { "top": 1, "right": 2, "bottom": 1, "left": 2 }, "style": { "background": [32, 30, 40], "foreground": [220, 218, 230] } }
  },
  "overlays": {
    "slash_command_dropdown": { "enabled_when": "!state.PopupOpen && len(state.SlashCommands()) > 0", "width": { "preferred": 44, "min": 20, "max": "state.width - 2" }, "style": { "background": [30, 27, 38], "foreground": [232, 229, 241], "muted": [159, 153, 176], "accent": [247, 184, 90] } },
    "command_palette_popup": { "enabled_when": "state.PopupOpen && (state.EffectiveLayout() != split || state.width < MinSplitLayoutWidth)" }
  }
}`
