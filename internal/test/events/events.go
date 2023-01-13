package events

const TestSource = "tag:thenativeweb.io,2023:eventsourcingdb:test"

type RegisteredEventData struct {
	Name string `json:"name"`
}

type RegisteredEvent struct {
	Type string
	Data RegisteredEventData
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
	Type string
	Data LoggedInEventData
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
		JaneDoe:   RegisteredEvent{PrefixEventType("registered"), RegisteredEventData{"Jane Doe"}},
		JohnDoe:   RegisteredEvent{PrefixEventType("registered"), RegisteredEventData{"John Doe"}},
		ApfelFred: RegisteredEvent{PrefixEventType("registered"), RegisteredEventData{"Apfel Fred"}},
	},
	LoggedIn: loggedInEvents{
		JaneDoe: LoggedInEvent{PrefixEventType("loggedIn"), LoggedInEventData{"Jane Doe"}},
		JohnDoe: LoggedInEvent{PrefixEventType("loggedIn"), LoggedInEventData{"Jane Doe"}},
	},
}
