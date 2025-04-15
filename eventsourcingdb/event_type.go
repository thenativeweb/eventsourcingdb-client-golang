package eventsourcingdb

type EventType struct {
	EventType string
	IsPhantom bool
	Schema    *map[string]any
}
