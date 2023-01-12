package eventsourcingdb

import (
	"encoding/json"
	"github.com/thenativeweb/eventsourcingdb-client-golang/timestamp"
	"net/url"
)

type EventContext struct {
	ID              string              `json:"id"`
	Time            timestamp.Timestamp `json:"time"`
	Subject         string              `json:"subject"`
	Type            string              `json:"type"`
	Source          url.URL             `json:"source"`
	PredecessorHash string              `json:"predecessorHash"`
	DataContentType string              `json:"datacontenttype"`
	SpecVersion     string              `json:"specversion"`
}

type Event struct {
	EventContext
	Data json.RawMessage `json:"data"`
}
