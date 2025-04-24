package eventsourcingdb_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestPing(t *testing.T) {
	t.Run("does not return an error if the server is reachable", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		err = client.Ping()
		assert.NoError(t, err)
	})

	t.Run("returns an error if the server is not reachable", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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

		err = client.Ping()
		assert.Error(t, err)
	})
}
