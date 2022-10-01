package eventsourcingdb

import (
	"errors"
	"fmt"
	"net/http"
)

func (client *Client) validateProtocolVersion(response *http.Response) error {
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
		client.configuration.protocolVersion.String(),
	)

	return errors.New(errorMessage)
}
