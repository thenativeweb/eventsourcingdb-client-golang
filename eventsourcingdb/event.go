package eventsourcingdb

import "encoding/json"

type Event struct {
	Source          string          `json:"source"`
	Subject         string          `json:"subject"`
	Type            string          `json:"type"`
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Time            Timestamp       `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
	PredecessorHash string          `json:"predecessorhash"`
	TraceParent     *string         `json:"traceparent,omitempty"`
	TraceState      *string         `json:"tracestate,omitempty"`
}

type EventContext struct {
	Source          string    `json:"source"`
	Subject         string    `json:"subject"`
	Type            string    `json:"type"`
	SpecVersion     string    `json:"specversion"`
	ID              string    `json:"id"`
	Time            Timestamp `json:"time"`
	DataContentType string    `json:"datacontenttype"`
	PredecessorHash string    `json:"predecessorhash"`
	TraceParent     *string   `json:"traceparent,omitempty"`
	TraceState      *string   `json:"tracestate,omitempty"`
}
