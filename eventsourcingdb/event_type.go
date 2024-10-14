package eventsourcingdb

import (
	"fmt"
	"regexp"
)

var eventTypeRegex *regexp.Regexp

func init() {
	eventTypeRegex = regexp.MustCompile(`^[0-9A-Za-z_-]{2,}\.([0-9A-Za-z_-]+\.)+[0-9A-Za-z_-]+$`)
}

func validateEventType(eventType string) error {
	didMatch := eventTypeRegex.MatchString(eventType)
	if !didMatch {
		return fmt.Errorf("malformed event type '%s': type must be a reverse domain name", eventType)
	}

	return nil
}
