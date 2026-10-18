package eventsourcingdb_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	t.Run("speaks the API that the server offers", func(t *testing.T) {
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

		api := "v1"
		if response.StatusCode == http.StatusOK {
			api = "v2"
		}

		client, err := container.GetClient(ctx)
		require.NoError(t, err)
		assert.Empty(t, eventsourcingdb.APIOf(client))

		err = client.Ping(ctx)
		require.NoError(t, err)

		assert.Equal(t, api, eventsourcingdb.APIOf(client))
		t.Logf("the client speaks the %s API with the image %s", api, imageTag)
	})
}
