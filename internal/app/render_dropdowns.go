package app

import "github.com/Ceinl/plums/internal/ui/tui/theme"

// dropdownFrame is the shared geometry and palette for every editor dropdown
// (skills, file paths, slash commands), so they all look and behave the same.
type dropdownFrame struct {
	x, y, w  int
	bg       string
	fg       string
	muted    string
	accent   string
	activeBg string
}

// newDropdownFrame computes placement for a dropdown with the given number of
// item rows (plus one header row) near the editor cursor. It returns false when
// there is no room to show the dropdown.
func newDropdownFrame(state *State, cfg *RenderConfig, rows int) (dropdownFrame, bool) {
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
		return dropdownFrame{}, false
	}
	x := cx
	if x+w > state.width {
		x = state.width - w
	}
	if x < 0 {
		x = 0
	}
	y := cy + 1
	if y+rows+1 > state.height {
		y = cy - rows - 1
	}
	if y < 0 {
		return dropdownFrame{}, false
	}

	frame := dropdownFrame{
		x:        x,
		y:        y,
		w:        w,
		bg:       ansiBgColor(overlay.Style.Background, overlay.Style.BackgroundToken, theme.BgSurface),
		fg:       ansiFgColor(overlay.Style.Foreground, overlay.Style.ForegroundToken, theme.Text),
		muted:    ansiFgColor(overlay.Style.Muted, overlay.Style.MutedToken, theme.TextMuted),
		accent:   ansiFgColor(overlay.Style.Accent, overlay.Style.AccentToken, theme.Accent),
		activeBg: theme.BgSelected.Bg(),
	}
	for row := 0; row < rows+1 && y+row < state.height; row++ {
		for col := 0; col < w; col++ {
			scr.Set(x+col, y+row, ' ', frame.fg, frame.bg, "")
		}
	}
	return frame, true
}

// drawRow renders one dropdown item row: an accented name, an optional muted
// detail, and the selected-row background when active.
func (f dropdownFrame) drawRow(i int, active bool, name, detail string) {
	row := f.y + i + 1
	rowBg := f.bg
	if active {
		rowBg = f.activeBg
		drawOverlayFill(f.x, row, f.w, rowBg)
	}
	drawOverlayText(f.x+1, row, f.w-2, name, f.accent, rowBg)
	detailX := f.x + 2 + len(name)
	if detail != "" && detailX < f.x+f.w-1 {
		drawOverlayText(detailX, row, f.x+f.w-detailX-1, detail, f.muted, rowBg)
	}
}

func renderEditorDropdown(state *State, cfg *RenderConfig) {
	if !overlayEnabled(state, cfg, "slash_command_dropdown") {
		return
	}
	if suggestions := state.SkillSuggestions(); len(suggestions) > 0 {
		renderSkillDropdown(state, cfg, suggestions)
		return
	}
	if suggestions := state.FileCommandSuggestions(); len(suggestions) > 0 {
		renderFileCommandDropdown(state, cfg, suggestions)
		return
	}
	renderSlashCommandDropdown(state, cfg)
}

func renderFileCommandDropdown(state *State, cfg *RenderConfig, suggestions []FileCommandSuggestion) {
	frame, ok := newDropdownFrame(state, cfg, len(suggestions))
	if !ok {
		return
	}
	drawOverlayText(frame.x+1, frame.y, frame.w-2, "file paths", frame.muted, frame.bg)
	active := state.ActiveEditorDropdownIndex(len(suggestions))
	for i, suggestion := range suggestions {
		frame.drawRow(i, i == active, "@"+suggestion.Path, "")
	}
}

func renderSlashCommandDropdown(state *State, cfg *RenderConfig) {
	commands := state.SlashCommands()
	if len(commands) == 0 {
		return
	}
	frame, ok := newDropdownFrame(state, cfg, len(commands))
	if !ok {
		return
	}
	drawOverlayText(frame.x+1, frame.y, frame.w-2, "slash commands", frame.muted, frame.bg)
	active := state.ActiveEditorDropdownIndex(len(commands))
	for i, command := range commands {
		frame.drawRow(i, i == active, command.Name, command.Detail)
	}
}

func renderSkillDropdown(state *State, cfg *RenderConfig, skills []SkillSuggestion) {
	visible := len(skills)
	if visible > 8 {
		visible = 8
	}
	frame, ok := newDropdownFrame(state, cfg, visible)
	if !ok {
		return
	}
	drawOverlayText(frame.x+1, frame.y, frame.w-2, "skills", frame.muted, frame.bg)
	active := state.ActiveEditorDropdownIndex(len(skills))
	start := active - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > len(skills) {
		start = len(skills) - visible
	}
	for i := 0; i < visible; i++ {
		idx := start + i
		skill := skills[idx]
		frame.drawRow(i, idx == active, "/skill "+skill.Name, skill.Description)
	}
}
