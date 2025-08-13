package eventsourcingdb_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestVerifyHash(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("verifies the event hash", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		err = container.Start(ctx)
		require.NoError(t, err)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		writtenEvents, err := client.WriteEvents(
			[]eventsourcingdb.EventCandidate{event},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)

		writtenEvent := writtenEvents[0]
		err = writtenEvent.VerifyHash()
		assert.NoError(t, err)
	})

	t.Run("returns an error if the event hash is invalid", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		err = container.Start(ctx)
		require.NoError(t, err)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		writtenEvents, err := client.WriteEvents(
			[]eventsourcingdb.EventCandidate{event},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)

		invalidHash := sha256.Sum256([]byte("invalid hash"))
		invalidHashHex := fmt.Sprintf("%x", invalidHash)

		writtenEvent := writtenEvents[0]
		writtenEvent.Hash = invalidHashHex

		err = writtenEvent.VerifyHash()
		assert.Error(t, err)
	})
}

func TestVerifySignature(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("returns an error if the signature is nil", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)

		err = container.Start(ctx)
		require.NoError(t, err)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		writtenEvents, err := client.WriteEvents(
			[]eventsourcingdb.EventCandidate{event},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)

		writtenEvent := writtenEvents[0]
		require.Nil(t, writtenEvent.Signature)

		publicKey, _, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		err = writtenEvent.VerifySignature(publicKey)
		assert.Error(t, err)
	})

	t.Run("returns an error if the hash verification fails", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().
			WithImageTag(imageVersion).
			WithSigningKey()

		err = container.Start(ctx)
		require.NoError(t, err)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		writtenEvents, err := client.WriteEvents(
			[]eventsourcingdb.EventCandidate{event},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)

		invalidHash := sha256.Sum256([]byte("invalid hash"))
		invalidHashHex := fmt.Sprintf("%x", invalidHash)

		writtenEvent := writtenEvents[0]
		require.NotNil(t, writtenEvent.Signature)

		writtenEvent.Hash = invalidHashHex

		verificationKey, err := container.GetVerificationKey()
		require.NoError(t, err)

		err = writtenEvent.VerifySignature(*verificationKey)
		assert.Error(t, err)
	})

	t.Run("returns an error if the signature verification fails", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().
			WithImageTag(imageVersion).
			WithSigningKey()

		err = container.Start(ctx)
		require.NoError(t, err)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		writtenEvents, err := client.WriteEvents(
			[]eventsourcingdb.EventCandidate{event},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)

		writtenEvent := writtenEvents[0]
		require.NotNil(t, writtenEvent.Signature)

		invalidSignature := *writtenEvent.Signature
		invalidSignature = invalidSignature + "0123456789abcdef"
		writtenEvent.Signature = &invalidSignature

		verificationKey, err := container.GetVerificationKey()
		require.NoError(t, err)

		err = writtenEvent.VerifySignature(*verificationKey)
		assert.Error(t, err)
	})

	t.Run("verifies the signature", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().
			WithImageTag(imageVersion).
			WithSigningKey()

		err = container.Start(ctx)
		require.NoError(t, err)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		writtenEvents, err := client.WriteEvents(
			[]eventsourcingdb.EventCandidate{event},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)

		writtenEvent := writtenEvents[0]
		require.NotNil(t, writtenEvent.Signature)

		verificationKey, err := container.GetVerificationKey()
		require.NoError(t, err)

		err = writtenEvent.VerifySignature(*verificationKey)
		assert.NoError(t, err)
	})
}
