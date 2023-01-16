package events

func PrefixEventType(eventSuffix string) string {
	return "io.thenativeweb.eventsourcingdb-client-golang.test." + eventSuffix
}
