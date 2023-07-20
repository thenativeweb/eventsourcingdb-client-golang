package events

import (
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"go.opentelemetry.io/otel/trace"
)

const TestSource = "tag:thenativeweb.io,2023:eventsourcingdb:test"

type RegisteredEventData struct {
	Name string `json:"name"`
}

type RegisteredEvent struct {
	Type           string
	Data           RegisteredEventData
	TracingContext *event.TracingContext
}

type registeredEvents struct {
	JaneDoe   RegisteredEvent
	JohnDoe   RegisteredEvent
	ApfelFred RegisteredEvent
}

type LoggedInEventData struct {
	Name string `json:"name"`
}

type LoggedInEvent struct {
	Type           string
	Data           LoggedInEventData
	TracingContext *event.TracingContext
}

type loggedInEvents struct {
	JaneDoe LoggedInEvent
	JohnDoe LoggedInEvent
}

type events struct {
	Registered registeredEvents
	LoggedIn   loggedInEvents
}

var Events = events{
	Registered: registeredEvents{
		JaneDoe: RegisteredEvent{PrefixEventType("registered"), RegisteredEventData{"Jane Doe"}, &event.TracingContext{
			TraceID:    ignoreError(trace.TraceIDFromHex("10000000000000000000000000000000")),
			SpanID:     ignoreError(trace.SpanIDFromHex("1000000000000000")),
			TraceFlags: 0,
			TraceState: trace.TraceState{},
		}},
		JohnDoe: RegisteredEvent{PrefixEventType("registered"), RegisteredEventData{"John Doe"}, &event.TracingContext{
			TraceID:    ignoreError(trace.TraceIDFromHex("20000000000000000000000000000000")),
			SpanID:     ignoreError(trace.SpanIDFromHex("2000000000000000")),
			TraceFlags: 0,
			TraceState: trace.TraceState{},
		}},
		ApfelFred: RegisteredEvent{PrefixEventType("registered"), RegisteredEventData{"Apfel Fred"}, &event.TracingContext{
			TraceID:    ignoreError(trace.TraceIDFromHex("30000000000000000000000000000000")),
			SpanID:     ignoreError(trace.SpanIDFromHex("3000000000000000")),
			TraceFlags: 0,
			TraceState: trace.TraceState{},
		}},
	},
	LoggedIn: loggedInEvents{
		JaneDoe: LoggedInEvent{PrefixEventType("loggedIn"), LoggedInEventData{"Jane Doe"}, &event.TracingContext{
			TraceID:    ignoreError(trace.TraceIDFromHex("40000000000000000000000000000000")),
			SpanID:     ignoreError(trace.SpanIDFromHex("4000000000000000")),
			TraceFlags: 0,
			TraceState: trace.TraceState{},
		}},
		JohnDoe: LoggedInEvent{PrefixEventType("loggedIn"), LoggedInEventData{"John Doe"}, &event.TracingContext{
			TraceID:    ignoreError(trace.TraceIDFromHex("50000000000000000000000000000000")),
			SpanID:     ignoreError(trace.SpanIDFromHex("5000000000000000")),
			TraceFlags: 0,
			TraceState: trace.TraceState{},
		}},
	},
}

func ignoreError[T any](value T, err error) T {
	return value
}
