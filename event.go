package eventsourcingdb

import (
	"encoding/json"
	"time"
)

type EventMetadata struct {
	ID              int64     `json:"id"`
	Timestamp       time.Time `json:"timestamp"`
	StreamName      string    `json:"streamName"`
	Name            string    `json:"name"`
	PredecessorHash string    `json:"predecessorHash"`
}

type Event struct {
	Metadata EventMetadata   `json:"metadata"`
	Data     json.RawMessage `json:"data"`
}
