package eventsourcingdbtest_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestContainer(t *testing.T) {
	t.Run("starts, hands out a client, and stops", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)
		assert.False(t, container.IsRunning())

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.True(t, container.IsRunning())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.Ping()
		assert.NoError(t, err)

		host, err := container.GetHost(t.Context())
		require.NoError(t, err)
		port, err := container.GetMappedPort(t.Context())
		require.NoError(t, err)
		baseURL, err := container.GetBaseURL(t.Context())
		require.NoError(t, err)

		assert.Equal(t, fmt.Sprintf("http://%s:%d", host, port), baseURL.String())
		assert.Equal(t, "secret", container.GetAPIToken())

		err = container.Stop(t.Context())
		require.NoError(t, err)
		assert.False(t, container.IsRunning())

		// Stopping a container that is not running is fine.
		err = container.Stop(t.Context())
		assert.NoError(t, err)
	})

	t.Run("uses a custom API token and port", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageVersion).
			WithPort(4000).
			WithAPIToken("custom-token")

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.Equal(t, "custom-token", container.GetAPIToken())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.VerifyAPIToken()
		assert.NoError(t, err)
	})

	t.Run("signs events with its signing key", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageVersion).
			WithSigningKey()

		signingKey, err := container.GetSigningKey()
		require.NoError(t, err)
		assert.NotNil(t, signingKey)

		verificationKey, err := container.GetVerificationKey()
		require.NoError(t, err)

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		written, err := client.WriteEvents([]eventsourcingdb.EventCandidate{{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data:    map[string]any{"value": 23},
		}}, nil)
		require.NoError(t, err)
		require.Len(t, written, 1)

		err = written[0].VerifySignature(*verificationKey)
		assert.NoError(t, err)
	})

	t.Run("fails to start with an image that does not exist", func(t *testing.T) {
		container := eventsourcingdbtest.NewContainer().WithImageTag("does-not-exist")

		err := container.Start(t.Context())
		assert.Error(t, err)
		assert.False(t, container.IsRunning())
	})

	t.Run("reports what it does not have", func(t *testing.T) {
		container := eventsourcingdbtest.NewContainer()

		_, err := container.GetHost(t.Context())
		assert.Error(t, err)
		_, err = container.GetMappedPort(t.Context())
		assert.Error(t, err)
		_, err = container.GetBaseURL(t.Context())
		assert.Error(t, err)
		_, err = container.GetClient(t.Context())
		assert.Error(t, err)

		_, err = container.GetSigningKey()
		assert.Error(t, err)
		_, err = container.GetVerificationKey()
		assert.Error(t, err)
	})
}
