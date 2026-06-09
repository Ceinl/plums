package app

import ()

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
	if y+len(suggestions)+1 > state.height {
		y = cy - len(suggestions) - 1
	}
	if y < 0 {
		return
	}

	bg := ansiBgColor(overlay.Style.Background, 30, 27, 38)
	fg := ansiFgColor(overlay.Style.Foreground, 232, 229, 241)
	muted := ansiFgColor(overlay.Style.Muted, 159, 153, 176)
	accent := ansiFgColor(overlay.Style.Accent, 247, 184, 90)
	activeBg := ansiBg(48, 43, 61)
	for row := 0; row < len(suggestions)+1 && y+row < state.height; row++ {
		for col := 0; col < w; col++ {
			scr.Set(x+col, y+row, ' ', fg, bg, "")
		}
	}
	drawOverlayText(x+1, y, w-2, "file paths", muted, bg)
	active := state.ActiveEditorDropdownIndex(len(suggestions))
	for i, suggestion := range suggestions {
		row := y + i + 1
		rowBg := bg
		if i == active {
			rowBg = activeBg
			drawOverlayFill(x, row, w, rowBg)
		}
		drawOverlayText(x+1, row, w-2, "@"+suggestion.Path, accent, rowBg)
	}
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
	activeBg := ansiBg(48, 43, 61)
	for row := 0; row < len(commands)+1 && y+row < state.height; row++ {
		for col := 0; col < w; col++ {
			scr.Set(x+col, y+row, ' ', fg, bg, "")
		}
	}
	drawOverlayText(x+1, y, w-2, "slash commands", muted, bg)
	active := state.ActiveEditorDropdownIndex(len(commands))
	for i, command := range commands {
		row := y + i + 1
		rowBg := bg
		if i == active {
			rowBg = activeBg
			drawOverlayFill(x, row, w, rowBg)
		}
		drawOverlayText(x+1, row, w-2, command.Name, accent, rowBg)
		detailX := x + 2 + len(command.Name)
		if detailX < x+w-1 {
			drawOverlayText(detailX, row, x+w-detailX-1, command.Detail, muted, rowBg)
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
	activeBg := ansiBg(48, 43, 61)
	for row := 0; row < visible+1 && y+row < state.height; row++ {
		for col := 0; col < w; col++ {
			scr.Set(x+col, y+row, ' ', fg, bg, "")
		}
	}
	drawOverlayText(x+1, y, w-2, "skills", muted, bg)
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
		row := y + i + 1
		name := "/skill " + skill.Name
		rowBg := bg
		if idx == active {
			rowBg = activeBg
			drawOverlayFill(x, row, w, rowBg)
		}
		drawOverlayText(x+1, row, w-2, name, accent, rowBg)
		detailX := x + 2 + len(name)
		if detailX < x+w-1 {
			drawOverlayText(detailX, row, x+w-detailX-1, skill.Description, muted, rowBg)
		}
	}
}
