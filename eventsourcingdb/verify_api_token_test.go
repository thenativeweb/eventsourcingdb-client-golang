package eventsourcingdb_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestVerifyAPIToken(t *testing.T) {
	t.Run("does not return an error if the token is valid", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		err = client.VerifyAPIToken()
		assert.NoError(t, err)
	})

	t.Run("returns an error if the token is invalid", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(context.Background())
		defer container.Stop(context.Background())

		baseURL, err := container.GetBaseURL(ctx)
		require.NoError(t, err)

		invalidToken := fmt.Sprintf("%s-invalid", container.GetAPIToken())

		client, err := eventsourcingdb.NewClient(baseURL, invalidToken)
		require.NoError(t, err)

		err = client.VerifyAPIToken()
		assert.Error(t, err)
	})
}
