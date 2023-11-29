package httputil_test

import (
	"net/http"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
)

func TestAddProtocolVersion(t *testing.T) {
	t.Run("adds the protocol version to the request.", func(t *testing.T) {
		request, err := http.NewRequest("GET", "http://localhost", nil)
		assert.NoError(t, err)

		httputil.AddProtocolVersion(request, *semver.MustParse("1.2.3"))

		assert.Equal(t, "1.2.3", request.Header.Get(httputil.ProtocolVersionHeader))
	})
}
