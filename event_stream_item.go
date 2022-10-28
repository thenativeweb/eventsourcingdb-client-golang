package eventsourcingdb

import "encoding/json"

type eventStreamItem struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
