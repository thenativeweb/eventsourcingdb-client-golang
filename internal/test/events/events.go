package events

const TestSource = "tag:thenativeweb.io,2023:eventsourcingdb:test"

type RegisteredEventData struct {
	Name string `json:"name"`
}

type RegisteredEvent struct {
	Type        string
	Data        RegisteredEventData
	TraceParent string
	TraceState  string
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
	Type        string
	Data        LoggedInEventData
	TraceParent string
	TraceState  string
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
		JaneDoe: RegisteredEvent{
			Type:        PrefixEventType("registered"),
			Data:        RegisteredEventData{"Jane Doe"},
			TraceParent: "00-10000000000000000000000000000000-1000000000000000-00",
		},
		JohnDoe: RegisteredEvent{
			Type:        PrefixEventType("registered"),
			Data:        RegisteredEventData{"John Doe"},
			TraceParent: "00-20000000000000000000000000000000-2000000000000000-00",
		},
		ApfelFred: RegisteredEvent{
			Type:        PrefixEventType("registered"),
			Data:        RegisteredEventData{"Apfel Fred"},
			TraceParent: "00-30000000000000000000000000000000-3000000000000000-00",
		},
	},
	LoggedIn: loggedInEvents{
		JaneDoe: LoggedInEvent{
			Type:        PrefixEventType("loggedIn"),
			Data:        LoggedInEventData{"Jane Doe"},
			TraceParent: "00-40000000000000000000000000000000-4000000000000000-00",
		},
		JohnDoe: LoggedInEvent{
			Type:        PrefixEventType("loggedIn"),
			Data:        LoggedInEventData{"John Doe"},
			TraceParent: "00-50000000000000000000000000000000-5000000000000000-00",
		},
	},
}
