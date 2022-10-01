package test

type registeredEventData struct {
	Name string `json:"name"`
}

type registeredEvent struct {
	Name string
	Data registeredEventData
}

type registeredEvents struct {
	JaneDoe registeredEvent
	JohnDoe registeredEvent
}

type events struct {
	Registered registeredEvents
}

var Events = events{
	Registered: registeredEvents{
		JaneDoe: registeredEvent{"registered", registeredEventData{"Jane Doe"}},
		JohnDoe: registeredEvent{"registered", registeredEventData{"John Doe"}},
	},
}
