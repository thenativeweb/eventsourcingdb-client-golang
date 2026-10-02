package eventsourcingdb

import (
	"errors"
	"net/http"
	"strings"
)

// ErrInvalidServerHeader reports that the server is not an EventSourcingDB,
// because its response lacks the Server header that starts with
// "EventSourcingDB/". This happens, e.g., if the URL points to another
// service, or to a proxy that answers in place of the database. Every
// function of the client that sends a request returns it in that case. Use
// errors.Is to check for it, which also finds it if it is wrapped:
//
//	err := client.Ping()
//	if errors.Is(err, eventsourcingdb.ErrInvalidServerHeader) {
//		// ...
//	}
var ErrInvalidServerHeader = errors.New("server must be EventSourcingDB")

func validateServerHeader(response *http.Response) error {
	serverHeader := response.Header.Get("Server")

	if serverHeader == "" || !strings.HasPrefix(serverHeader, "EventSourcingDB/") {
		return ErrInvalidServerHeader
	}

	return nil
}
