package eventsourcingdb

// EventType is an event type that the database knows, because it has events of
// that type, a schema registered for it, or both.
type EventType struct {
	// Type is the name of the event type, e.g.
	// "io.eventsourcingdb.library.book-acquired".
	Type string

	// HasEvents tells whether the database has events of that type. An event
	// type without events has a schema registered for it.
	HasEvents bool

	// Schema is the schema registered for the event type, or nil if there is
	// none.
	Schema *map[string]any
}
