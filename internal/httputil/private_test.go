package httputil

import (
	"net/http"

	"github.com/Masterminds/semver"
)

func AddProtocolVersion(request *http.Request, version semver.Version) {
	addProtocolVersion(request, version)
}

func AddAccessToken(request *http.Request, accessToken string) {
	addAccessToken(request, accessToken)
}
