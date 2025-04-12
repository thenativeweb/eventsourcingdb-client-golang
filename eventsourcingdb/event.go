package eventsourcingdb

import (
	"encoding/json"
	"time"
)

type Event struct {
	SpecVersion     string
	ID              string
	Time            time.Time
	Source          string
	Subject         string
	Type            string
	DataContentType string
	Data            json.RawMessage
	Hash            string
	PredecessorHash string
	TraceParent     *string
	TraceState      *string
}
