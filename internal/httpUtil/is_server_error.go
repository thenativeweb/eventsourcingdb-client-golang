package httpUtil

import "net/http"

// IsServerError returns true if the HTTP status code is in the 5xx range.
func IsServerError(response *http.Response) bool {
	return response.StatusCode >= 500 && response.StatusCode < 600
}
