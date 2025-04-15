package internal

type StreamEventType struct {
	EventType string          `json:"eventType"`
	IsPhantom bool            `json:"isPhantom"`
	Schema    *map[string]any `json:"schema,omitempty"`
}
