package test

type RegisteredEventData struct {
	Name string `json:"name"`
}

type registeredEvent struct {
	Name string
	Data RegisteredEventData
}

type registeredEvents struct {
	JaneDoe registeredEvent
	JohnDoe registeredEvent
}

type LoggedInEventData struct {
	Name string `json:"name"`
}

type loggedInEvent struct {
	Name string
	Data LoggedInEventData
}

type loggedInEvents struct {
	JaneDoe loggedInEvent
	JohnDoe loggedInEvent
}

type events struct {
	Registered registeredEvents
	LoggedIn   loggedInEvents
}

var Events = events{
	Registered: registeredEvents{
		JaneDoe: registeredEvent{"registered", RegisteredEventData{"Jane Doe"}},
		JohnDoe: registeredEvent{"registered", RegisteredEventData{"John Doe"}},
	},
	LoggedIn: loggedInEvents{
		JaneDoe: loggedInEvent{"loggedIn", LoggedInEventData{"Jane Doe"}},
		JohnDoe: loggedInEvent{"loggedIn", LoggedInEventData{"Jane Doe"}},
	},
}
