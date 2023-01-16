package ndjson

import "encoding/json"

type StreamItem struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
