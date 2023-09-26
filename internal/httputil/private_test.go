package httputil

import (
	"github.com/Masterminds/semver"
	"net/http"
)

func AddProtocolVersion(request *http.Request, version semver.Version) {
	addProtocolVersion(request, version)
}

func AddAccessToken(request *http.Request, accessToken string) {
	addAccessToken(request, accessToken)
}
