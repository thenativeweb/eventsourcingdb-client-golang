package httputil_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
)

func TestAddAccessToken(t *testing.T) {
	t.Run("adds the access token to the request.", func(t *testing.T) {
		request, err := http.NewRequest("GET", "http://localhost", nil)
		assert.NoError(t, err)

		httputil.AddAccessToken(request, "secret")

		assert.Equal(t, "Bearer secret", request.Header.Get("Authorization"))
	})

	t.Run("does not add an access token if the access token is empty.", func(t *testing.T) {
		request, err := http.NewRequest("GET", "http://localhost", nil)
		assert.NoError(t, err)

		httputil.AddAccessToken(request, "")

		assert.Equal(t, "", request.Header.Get("Authorization"))
	})
}
