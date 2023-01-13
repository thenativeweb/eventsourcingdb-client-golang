package event

import (
	"fmt"
	"net/url"
)

func ValidateSource(source string) error {
	if _, err := url.Parse(source); err != nil {
		return fmt.Errorf("malformed event source: %w", err)
	}

	return nil
}
