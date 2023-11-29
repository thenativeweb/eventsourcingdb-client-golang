package event

import "encoding/json"

type Context struct {
	CandidateContext
	SpecVersion     string    `json:"specversion"`
	ID              string    `json:"id"`
	Time            Timestamp `json:"time"`
	DataContentType string    `json:"datacontenttype"`
	PredecessorHash string    `json:"predecessorhash"`
}

type Event struct {
	Context
	Data json.RawMessage `json:"data"`
}
