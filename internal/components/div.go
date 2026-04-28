package components

import (
	"plums/internal/layout"
	"plums/internal/screen"
)

type Div struct {
	isDirty bool

	X, Y int
	W, H layout.Unit

	justifyContent layout.JustifyContent
	alignItems     layout.AlignItems

	children []layout.Component

	parent layout.Component
}

func (d *Div) IsDirty() bool {
	return d.isDirty
}

func (d *Div) MakeDirty() {
	d.isDirty = true
}

func (d *Div) ClearDirty() {
	d.isDirty = false
}

func (d *Div) Layout() {
	if d.parent == nil { // Only root div will not have a parent, use full screen
		d.W = layout.Unit{
			Type:  layout.UnitPersent,
			Value: 100,
		}
		d.H = layout.Unit{
			Type:  layout.UnitPersent,
			Value: 100,
		}

		d.X = 0
		d.Y = 0
		return
	}
	d.X = d.parent.(*Div).X + int(d.parent.(*Div).W.Value)
	d.Y = d.parent.(*Div).Y + int(d.parent.(*Div).H.Value)
}

func (d *Div) SetSize(w, h layout.Unit) {
	d.W = w
	d.H = h
}

func (d *Div) Render(screen *screen.Screen) {
	// Div is just empty box, so nothing to render YET,
	// TODO: Add borders and background
	for _, child := range d.children {
		child.Render(screen)
	}
}

func (d *Div) JustifyContent(jc layout.JustifyContent) {
	d.justifyContent = jc
}

func (d *Div) AlignItems(ai layout.AlignItems) {
	d.alignItems = ai
}

func (d *Div) SetParent(parent layout.Component) {
	d.parent = parent
}

func (d *Div) AppendChild(children layout.Component) {
	children.SetParent(d)
	d.children = append(d.children, children)
}
