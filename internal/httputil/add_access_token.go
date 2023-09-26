package httputil

import (
	"net/http"
)

func addAccessToken(request *http.Request, accessToken string) {
	if accessToken == "" {
		return
	}

	request.Header.Add("Authorization", "Bearer "+accessToken)
}
