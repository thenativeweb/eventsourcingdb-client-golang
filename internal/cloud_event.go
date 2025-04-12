package internal

import "encoding/json"

type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Time            string          `json:"time"`
	Source          string          `json:"source"`
	Subject         string          `json:"subject"`
	Type            string          `json:"type"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
	Hash            string          `json:"hash"`
	PredecessorHash string          `json:"predecessorhash"`
	TraceParent     *string         `json:"traceparent"`
	TraceState      *string         `json:"tracestate"`
}
