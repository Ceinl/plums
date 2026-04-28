package layout

import (
	"plums/internal/screen"
)

// Main interface for all layout components
type Component interface {
	IsDirty() bool
	MakeDirty()
	ClearDirty()

	Layout()
	Render(screen *screen.Screen)

	SetParent(parent Component)
}
