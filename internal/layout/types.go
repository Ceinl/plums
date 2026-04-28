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
	ALeft
	ARight
)

type Unit struct {
	Type  UnitType
	Value float64
}

func (u Unit) Resolve(total int) int {
	switch u.Type {
	case UnitPx:
		return int(u.Value)
	case UnitPersent:
		return int(float64(total) * u.Value / 100.0)
	case UnitGrow:
		return 0
	}
	return 0
}

type Padding struct {
	Top    Unit
	Bottom Unit
	Left   Unit
	Right  Unit
}
