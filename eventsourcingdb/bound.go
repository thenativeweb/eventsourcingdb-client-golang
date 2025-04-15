package eventsourcingdb

type BoundType string

const (
	BoundTypeInclusive BoundType = "inclusive"
	BoundTypeExclusive BoundType = "exclusive"
)

type Bound struct {
	ID   string
	Type BoundType
}
