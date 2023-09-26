package httputil

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver"
	"net/http"
)

func validateProtocolVersion(response *http.Response, clientProtocolVersion semver.Version) error {
	if response.StatusCode != http.StatusUnprocessableEntity {
		return nil
	}

	serverProtocolVersion := response.Header.Get("X-EventSourcingDB-Protocol-Version")
	if serverProtocolVersion == "" {
		serverProtocolVersion = "unknown version"
	}

	errorMessage := fmt.Sprintf(
		"protocol version mismatch, server '%s', client '%s'",
		serverProtocolVersion,
		clientProtocolVersion.String(),
	)

	return errors.New(errorMessage)
}
