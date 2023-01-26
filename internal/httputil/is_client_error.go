package httputil

import "net/http"

// IsClientError returns true if the HTTP status code is in the 4xx range.
func IsClientError(response *http.Response) bool {
	return response != nil && response.StatusCode >= 400 && response.StatusCode < 500
}
