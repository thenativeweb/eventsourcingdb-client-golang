package event

import (
	"fmt"
	"regexp"
)

var subjectRegex *regexp.Regexp

func init() {
	wordRegex := "[0-9A-Za-z_-]+"
	subjectRegex = regexp.MustCompile(fmt.Sprintf("^/(%[1]s/)*(%[1]s/?)?$", wordRegex))
}

func ValidateSubject(subject string) error {
	didMatch := subjectRegex.MatchString(subject)
	if !didMatch {
		return fmt.Errorf("malformed event subject '%s': subject must be an absolute, slash-separated path", subject)
	}

	return nil
}
