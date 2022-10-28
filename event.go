package eventsourcingdb

import (
	"encoding/json"
	"github.com/thenativeweb/eventsourcingdb-client-golang/timestamp"
)

type EventMetadata struct {
	ID              int64               `json:"id"`
	Timestamp       timestamp.Timestamp `json:"timestamp"`
	StreamName      string              `json:"streamName"`
	Name            string              `json:"name"`
	PredecessorHash string              `json:"predecessorHash"`
}

type Event struct {
	Metadata EventMetadata   `json:"metadata"`
	Data     json.RawMessage `json:"data"`
}
