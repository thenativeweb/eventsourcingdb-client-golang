package httputil

import (
	"github.com/Masterminds/semver"
	"net/http"
)

const ProtocolVersionHeader = "X-EventSourcingDB-Protocol-Version"

func addProtocolVersion(request *http.Request, version semver.Version) {
	request.Header.Add(ProtocolVersionHeader, version.String())
}
