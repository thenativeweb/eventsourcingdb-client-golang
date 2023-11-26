package httputil

import (
	"net/http"

	"github.com/Masterminds/semver"
)

const ProtocolVersionHeader = "X-EventSourcingDB-Protocol-Version"

func addProtocolVersion(request *http.Request, version semver.Version) {
	request.Header.Add(ProtocolVersionHeader, version.String())
}
