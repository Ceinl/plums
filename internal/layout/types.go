package layout

type UnitType int
type JustifyContent int
type AlignItems int

const (
	UnitPx UnitType = iota
	UnitPersent
	UnitGrow

	JCenter JustifyContent = iota
	JLeft
	JRight

	ACenter AlignItems = iota
	ATop
	ABottom
)

type Unit struct {
	Type  UnitType
	Value float64
}

type Padding struct {
	Top    Unit
	Bottom Unit
	Left   Unit
	Right  Unit
}
