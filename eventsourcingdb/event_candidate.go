package eventsourcingdb

type EventCandidate struct {
	Source      string
	Subject     string
	Type        string
	Data        any
	TraceParent *string
	TraceState  *string
}
