package eventsourcingdb_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
)

func TestPing(t *testing.T) {
	t.Run("does not return an error if the server is reachable", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		err = client.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("returns an error if the server is not reachable", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(context.Background())
		defer container.Stop(context.Background())

		port, err := container.GetMappedPort(ctx)
		require.NoError(t, err)

		baseURL, err := url.Parse(
			fmt.Sprintf("http://non-existent-host:%d", port),
		)
		require.NoError(t, err)

		apiToken := container.GetAPIToken()

		client, err := eventsourcingdb.NewClient(baseURL, apiToken)
		require.NoError(t, err)

		err = client.Ping(ctx)
		assert.Error(t, err)
	})
}

// The client has to speak the v2 API with a server that offers it, and the v1
// API with every other one. Since EventSourcingDB 1.3 still offers the v1 API
// as well, every other test passes against it with a client that wrongly
// speaks the v1 API, so this test tells which API the requests of the client
// go to, and asks the server itself which API it offers, whatever the tag of
// its image says.
func TestTheAPIOfTheServer(t *testing.T) {
	t.Run("every request goes to the API that the server offers", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		baseURL, err := container.GetBaseURL(ctx)
		require.NoError(t, err)

		// A server with the v2 API answers its ping, and one without it does
		// not know the path.
		response, err := http.Get(baseURL.JoinPath("/api/v2/system/ping").String())
		require.NoError(t, err)
		response.Body.Close()

		api, prefix := "v1", "/api/v1/"
		if response.StatusCode == http.StatusOK {
			api, prefix = "v2", "/api/v2/"
		}
		t.Logf("the server of the image %s offers the %s API", imageTag, api)

		// The client talks to the database through a proxy that records
		// every request it passes on.
		var mu sync.Mutex
		var requests []string

		proxy := httputil.NewSingleHostReverseProxy(baseURL)
		proxy.FlushInterval = -1
		proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requests = append(requests, r.Method+" "+r.URL.Path)
			mu.Unlock()

			proxy.ServeHTTP(w, r)
		}))
		defer proxyServer.Close()

		proxyURL, err := url.Parse(proxyServer.URL)
		require.NoError(t, err)

		client, err := eventsourcingdb.NewClient(proxyURL, container.GetAPIToken())
		require.NoError(t, err)

		for _, request := range requestsOf(client) {
			// Observing never ends, so it reads the event that the write
			// before wrote, and stops.
			if request.name == "ObserveEvents" {
				for _, err := range client.ObserveEvents(ctx, "/test", eventsourcingdb.ObserveEventsOptions{}) {
					require.NoError(t, err)
					break
				}
				continue
			}

			// The v1 API takes a schema only if it names its properties.
			if request.name == "RegisterEventSchema" {
				err := client.RegisterEventSchema(ctx, "io.eventsourcingdb.test", map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				})
				require.NoError(t, err)
				continue
			}

			require.NoError(t, request.send(), request.name)
		}

		assert.Equal(t, api, eventsourcingdb.APIOf(client))

		mu.Lock()
		defer mu.Unlock()

		require.Len(t, requests, len(requestsOf(nil))+1)
		assert.Equal(t, "GET /api/v2/system/ping", requests[0], "the client did not ask which API the server offers")
		for _, request := range requests[1:] {
			_, path, _ := strings.Cut(request, " ")
			assert.True(t, strings.HasPrefix(path, prefix), "%s does not go to the %s API", request, api)
		}
	})
}
