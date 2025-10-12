package internal

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func ValidateServerHeader(response *http.Response) error {
	serverHeader := response.Header.Get("Server")

	if serverHeader == "" {
		return errors.New("server must be EventSourcingDB, but Server header is missing")
	}

	if !strings.HasPrefix(serverHeader, "EventSourcingDB/") {
		return fmt.Errorf("server must be EventSourcingDB, got Server header: '%s'", serverHeader)
	}

	return nil
}
