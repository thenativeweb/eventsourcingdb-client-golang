package eventsourcingdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// A caller that observes events has to learn when the connection stalls, e.g.
// behind a proxy that keeps it open but no longer passes anything on.
// Otherwise it keeps believing it is live, while nothing arrives any more. The
// heartbeats that the server sends while there is nothing else to send must
// keep a stream alive, though.
func TestHeartbeats(t *testing.T) {
	timeout := 250 * time.Millisecond
	withTimeout := eventsourcingdb.WithStallTimeout(timeout)

	for _, api := range apis() {
		t.Run(api.name, func(t *testing.T) {
			t.Run("every stream with heartbeats stalls if nothing arrives after a heartbeat", func(t *testing.T) {
				for _, stream := range streamsWithHeartbeats(api) {
					t.Run(stream.name, func(t *testing.T) {
						connectionClosed := make(chan struct{})
						client := clientOfAServerThatStreams(t, api, func(ctx context.Context, send func(line string)) {
							send(api.heartbeat())

							select {
							case <-ctx.Done():
								close(connectionClosed)
							case <-time.After(5 * time.Second):
							}
						}, withTimeout)

						started := time.Now()
						_, err := stream.read(context.Background(), client, nil)
						elapsed := time.Since(started)

						assert.ErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
						assert.EqualError(t, err, "connection stalled, nothing arrived from the server for 250ms")
						assert.GreaterOrEqual(t, elapsed, timeout)
						assert.Less(t, elapsed, 2*time.Second, "the stall timeout did not end the stream")

						select {
						case <-connectionClosed:
						case <-time.After(time.Second):
							assert.Fail(t, "the client did not close the connection")
						}
					})
				}
			})

			t.Run("heartbeats keep a stream alive", func(t *testing.T) {
				for _, stream := range streamsWithHeartbeats(api) {
					t.Run(stream.name, func(t *testing.T) {
						// The heartbeats last for three times as long as the timeout.
						client := clientOfAServerThatStreams(t, api, func(ctx context.Context, send func(line string)) {
							for range 15 {
								send(api.heartbeat())
								time.Sleep(timeout / 5)
							}
							send(stream.item(api))
						}, withTimeout)

						items, err := stream.read(context.Background(), client, nil)

						assertEndedCleanly(t, api, stream, err)
						assert.Equal(t, 1, items)
					})
				}
			})

			t.Run("a cancel ends a stream as before, not as a stall", func(t *testing.T) {
				for _, stream := range streamsWithHeartbeats(api) {
					t.Run(stream.name, func(t *testing.T) {
						client := clientOfAServerThatStreams(t, api, func(ctx context.Context, send func(line string)) {
							send(api.heartbeat())

							select {
							case <-ctx.Done():
							case <-time.After(5 * time.Second):
							}
						}, withTimeout)

						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						go func() {
							time.Sleep(timeout / 5)
							cancel()
						}()

						_, err := stream.read(ctx, client, nil)

						assert.ErrorIs(t, err, context.Canceled)
						assert.NotErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
					})
				}
			})
		})
	}
}
