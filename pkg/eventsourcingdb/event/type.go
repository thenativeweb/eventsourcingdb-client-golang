package event

import (
	"fmt"
	"regexp"
)

var typeRegex *regexp.Regexp

func init() {
	typeRegex = regexp.MustCompile(`^[0-9A-Za-z_-]{2,}\.([0-9A-Za-z_-]+\.)+[0-9A-Za-z_-]+$`)
}

func ValidateType(eventType string) error {
	didMatch := typeRegex.MatchString(eventType)
	if !didMatch {
		return fmt.Errorf("malformed event type '%s', must be reverse domain name", eventType)
	}

	return nil
}
