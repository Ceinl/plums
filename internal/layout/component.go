package layout

import "plums/internal/screen"

// Main interface for all layout components
type Component interface {
	IsDirty() bool
	MakeDirty()
	ClearDirty()

	Layout(x, y, w, h int)
	Render(screen *screen.Screen)

	SetParent(parent Component)
}
