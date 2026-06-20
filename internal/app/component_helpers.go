package app

import (
	"fmt"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
)

// bgStyle builds a layout.Style whose background matches the ANSI truecolor
// background string resolved by the public component adapter (the slot's themed
// background). Ported display widgets read their background from the style when
// they have no layout parent, so this keeps panes painting on the same themed
// background the privileged factory path used.
func bgStyle(bg string) layout.Style {
	style := layout.Style{}
	var r, g, b uint8
	if _, err := fmt.Sscanf(bg, "\x1b[48;2;%d;%d;%dm", &r, &g, &b); err == nil {
		style.SetBackground(r, g, b)
	}
	return style
}
