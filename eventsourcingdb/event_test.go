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
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
)

func TestVerifyHash(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("verifies the event hash", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
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
			ctx,
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

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
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
			ctx,
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
		assert.ErrorIs(t, err, eventsourcingdb.ErrHashVerificationFailed)
		assert.EqualError(t, err, "hash verification failed")
	})
}

func TestVerifySignature(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("returns an error if the signature is nil", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)

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
			ctx,
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

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
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
			ctx,
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
		assert.ErrorIs(t, err, eventsourcingdb.ErrHashVerificationFailed)
		assert.NotErrorIs(t, err, eventsourcingdb.ErrSignatureVerificationFailed)
	})

	t.Run("returns an error if the signature verification fails", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
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
			ctx,
			[]eventsourcingdb.EventCandidate{event},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)

		writtenEvent := writtenEvents[0]
		require.NotNil(t, writtenEvent.Signature)

		validSignature := *writtenEvent.Signature
		tamperedSignature := validSignature + "0123456789abcdef"
		writtenEvent.Signature = &tamperedSignature

		verificationKey, err := container.GetVerificationKey()
		require.NoError(t, err)

		err = writtenEvent.VerifySignature(*verificationKey)
		assert.ErrorIs(t, err, eventsourcingdb.ErrSignatureVerificationFailed)
		assert.EqualError(t, err, "signature verification failed")
	})

	t.Run("returns an error if the signature belongs to another key", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
			WithSigningKey()

		err = container.Start(ctx)
		require.NoError(t, err)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				{
					Source:  "https://www.eventsourcingdb.io",
					Subject: "/test",
					Type:    "io.eventsourcingdb.test",
					Data: EventData{
						Value: 23,
					},
				},
			},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)

		otherVerificationKey, _, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		err = writtenEvents[0].VerifySignature(otherVerificationKey)
		wrapped := fmt.Errorf("failed to handle the event: %w", err)

		assert.ErrorIs(t, wrapped, eventsourcingdb.ErrSignatureVerificationFailed)
	})

	t.Run("verifies the signature", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
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
			ctx,
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

// A caller has to be able to verify every event the server hands out, whether
// it wrote, read, or observed it, with whichever version of the hash the
// server writes. The data of an event has to be hashed as the server sent it,
// so these events have data that JSON escapes, and a trace context, which the
// hash of version 2 covers.
func TestVerifyEventsOfTheServer(t *testing.T) {
	ctx := context.Background()

	imageTag, err := getImageTag()
	require.NoError(t, err)

	container := eventsourcingdbtest.NewContainer().
		WithImageTag(imageTag).
		WithSigningKey()

	err = container.Start(ctx)
	require.NoError(t, err)
	defer container.Stop(ctx)

	client, err := container.GetClient(ctx)
	require.NoError(t, err)

	verificationKey, err := container.GetVerificationKey()
	require.NoError(t, err)

	candidates := []eventsourcingdb.EventCandidate{
		{
			Source:  "https://library.eventsourcingdb.io",
			Subject: "/books/42",
			Type:    "io.eventsourcingdb.library.book-acquired",
			Data: map[string]any{
				"title":  "2001 – A Space Odyssey",
				"author": "Arthur C. Clarke",
				"note":   `<a & b> "quoted"`,
				"pages":  296,
			},
		},
		{
			Source:      "https://library.eventsourcingdb.io",
			Subject:     "/books/42",
			Type:        "io.eventsourcingdb.library.book-borrowed",
			Data:        map[string]any{"borrowedBy": "/readers/23"},
			TraceParent: new("00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"),
			TraceState:  new("rojo=00f067aa0ba902b7,congo=t61rcWkgMzE"),
		},
	}

	writtenEvents, err := client.WriteEvents(ctx, candidates, nil)
	require.NoError(t, err)

	var readEvents []eventsourcingdb.Event
	for event, err := range client.ReadEvents(ctx, "/books/42", eventsourcingdb.ReadEventsOptions{}) {
		require.NoError(t, err)
		readEvents = append(readEvents, event)
	}

	observeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var observedEvents []eventsourcingdb.Event
	for event, err := range client.ObserveEvents(observeCtx, "/books/42", eventsourcingdb.ObserveEventsOptions{}) {
		require.NoError(t, err)
		observedEvents = append(observedEvents, event)
		if len(observedEvents) == len(candidates) {
			break
		}
	}

	eventsBy := map[string][]eventsourcingdb.Event{
		"written":  writtenEvents,
		"read":     readEvents,
		"observed": observedEvents,
	}

	for how, events := range eventsBy {
		t.Run(how, func(t *testing.T) {
			require.Len(t, events, len(candidates))

			for _, event := range events {
				require.NotNil(t, event.Signature)

				assert.NoError(t, event.VerifyHash())
				assert.NoError(t, event.VerifySignature(*verificationKey))
			}
		})
	}
}
