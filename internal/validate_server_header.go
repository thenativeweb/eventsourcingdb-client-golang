package internal

import (
	"errors"
	"net/http"
	"strings"
)

func ValidateServerHeader(response *http.Response) error {
	serverHeader := response.Header.Get("Server")

	if serverHeader == "" || !strings.HasPrefix(serverHeader, "EventSourcingDB/") {
		return errors.New("server must be EventSourcingDB")
	}

	return nil
}
