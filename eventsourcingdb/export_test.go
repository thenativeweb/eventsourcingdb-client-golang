package eventsourcingdb

import (
	"net/http"
	"time"
)

// ConnectionStalledAfter returns the error with which a request ends if
// nothing arrives for the given stall timeout, so that tests need not wait for
// 30 seconds to see it.
func ConnectionStalledAfter(stallTimeout time.Duration) error {
	return &connectionStalledError{stallTimeout: stallTimeout}
}

// StallTimeoutOf returns the stall timeout of a client.
func StallTimeoutOf(client *Client) time.Duration {
	return client.stallTimeout
}

// HTTPClientOf returns the HTTP client that a client sends its requests with.
func HTTPClientOf(client *Client) *http.Client {
	return client.httpClient
}

// UseAPI makes a client speak the given API, "v1" or "v2", as if it had asked
// the server already, so that a test of a request need not answer that
// question.
func UseAPI(client *Client, version string) {
	client.apiChoice.mu.Lock()
	defer client.apiChoice.mu.Unlock()

	switch version {
	case "v1":
		client.apiChoice.version = apiVersionV1
	case "v2":
		client.apiChoice.version = apiVersionV2
	default:
		panic("unknown API " + version)
	}
}

// APIOf returns the API that a client speaks, "v1" or "v2", or an empty
// string if it has not asked the server yet.
func APIOf(client *Client) string {
	client.apiChoice.mu.Lock()
	defer client.apiChoice.mu.Unlock()

	switch client.apiChoice.version {
	case apiVersionV1:
		return "v1"
	case apiVersionV2:
		return "v2"
	default:
		return ""
	}
}

// SetWaitingForProbe makes a client call waiting whenever a request starts to
// wait for the answer to the question of another request which API the
// server offers.
func SetWaitingForProbe(client *Client, waiting func()) {
	client.apiChoice.mu.Lock()
	defer client.apiChoice.mu.Unlock()

	client.apiChoice.waiting = waiting
}
