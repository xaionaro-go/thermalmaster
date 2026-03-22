package thermalmaster

// Model represents a camera model.
type Model int

const (
	ModelP3 Model = iota
	ModelP1
)

func (m Model) String() string {
	switch m {
	case ModelP3:
		return "P3"
	case ModelP1:
		return "P1"
	default:
		return "unknown"
	}
}
