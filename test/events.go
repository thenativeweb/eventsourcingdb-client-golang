package test

type RegisteredEventData struct {
	Name string `json:"name"`
}

type RegisteredEvent struct {
	Name string
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
	Name string
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
		JaneDoe:   RegisteredEvent{"registered", RegisteredEventData{"Jane Doe"}},
		JohnDoe:   RegisteredEvent{"registered", RegisteredEventData{"John Doe"}},
		ApfelFred: RegisteredEvent{"registered", RegisteredEventData{"Apfel Fred"}},
	},
	LoggedIn: loggedInEvents{
		JaneDoe: LoggedInEvent{"loggedIn", LoggedInEventData{"Jane Doe"}},
		JohnDoe: LoggedInEvent{"loggedIn", LoggedInEventData{"Jane Doe"}},
	},
}
