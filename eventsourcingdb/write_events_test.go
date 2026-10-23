package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
)

func TestWriteEvents(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("writes a single event", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				event,
			},
			nil,
		)
		assert.NoError(t, err)
		assert.Len(t, writtenEvents, 1)
		assert.Equal(t, "0", writtenEvents[0].ID)
	})

	t.Run("writes the trace context of an event", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		traceParent := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
		traceState := "rojo=00f067aa0ba902b7"

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
			TraceParent: new(traceParent),
			TraceState:  new(traceState),
		}

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				event,
			},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)
		assert.Equal(t, &traceParent, writtenEvents[0].TraceParent)
		assert.Equal(t, &traceState, writtenEvents[0].TraceState)

		eventsRead := []eventsourcingdb.Event{}
		for event, err := range client.ReadEvents(
			ctx,
			"/test",
			eventsourcingdb.ReadEventsOptions{
				Recursive: false,
			},
		) {
			require.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		require.Len(t, eventsRead, 1)
		assert.Equal(t, &traceParent, eventsRead[0].TraceParent)
		assert.Equal(t, &traceState, eventsRead[0].TraceState)
	})

	t.Run("writes multiple events", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		assert.NoError(t, err)
		assert.Len(t, writtenEvents, 2)

		var eventData EventData

		assert.Equal(t, "0", writtenEvents[0].ID)
		err = json.Unmarshal(writtenEvents[0].Data, &eventData)
		assert.NoError(t, err)
		assert.Equal(t, 23, eventData.Value)

		assert.Equal(t, "1", writtenEvents[1].ID)
		err = json.Unmarshal(writtenEvents[1].Data, &eventData)
		assert.NoError(t, err)
		assert.Equal(t, 42, eventData.Value)
	})

	t.Run("supports the isSubjectPristine precondition", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectPristinePrecondition("/test"),
			},
		)

		assertCodeOfServer(t, err, "write events", eventsourcingdb.CodePreconditionViolated)
	})

	t.Run("supports the isSubjectPopulated precondition", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test"),
			},
		)
		assertCodeOfServer(t, err, "write events", eventsourcingdb.CodePreconditionViolated)

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test"),
			},
		)
		assert.NoError(t, err)
		assert.Len(t, writtenEvents, 1)
		assert.Equal(t, "1", writtenEvents[0].ID)

		var eventData EventData
		err = json.Unmarshal(writtenEvents[0].Data, &eventData)
		assert.NoError(t, err)
		assert.Equal(t, 42, eventData.Value)
	})

	t.Run("supports the isSubjectOnEventId precondition", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "1"),
			},
		)

		assertCodeOfServer(t, err, "write events", eventsourcingdb.CodePreconditionViolated)
	})

	t.Run("supports the isSubjectPristine precondition recursively", func(t *testing.T) {
		client := startedClient(t)
		writeEventTo(t, client, "/test/nested")

		assertPreconditionsOnSubject(t, client, []preconditionOnSubject{
			{precondition: eventsourcingdb.NewIsSubjectPristinePrecondition("/test", eventsourcingdb.PreconditionOptions{Recursive: false}), holds: true},
			{precondition: eventsourcingdb.NewIsSubjectPristinePrecondition("/test/nested", eventsourcingdb.PreconditionOptions{Recursive: false}), holds: false},
			{precondition: eventsourcingdb.NewIsSubjectPristinePrecondition("/other", eventsourcingdb.PreconditionOptions{Recursive: true}), isRecursive: true, holds: true},
			{precondition: eventsourcingdb.NewIsSubjectPristinePrecondition("/test", eventsourcingdb.PreconditionOptions{Recursive: true}), isRecursive: true, holds: false},
		})
	})

	t.Run("supports the isSubjectPopulated precondition recursively", func(t *testing.T) {
		client := startedClient(t)
		writeEventTo(t, client, "/test/nested")

		assertPreconditionsOnSubject(t, client, []preconditionOnSubject{
			{precondition: eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test/nested", eventsourcingdb.PreconditionOptions{Recursive: false}), holds: true},
			{precondition: eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test", eventsourcingdb.PreconditionOptions{Recursive: false}), holds: false},
			{precondition: eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test", eventsourcingdb.PreconditionOptions{Recursive: true}), isRecursive: true, holds: true},
			{precondition: eventsourcingdb.NewIsSubjectPopulatedPrecondition("/other", eventsourcingdb.PreconditionOptions{Recursive: true}), isRecursive: true, holds: false},
		})
	})

	t.Run("supports the isSubjectOnEventId precondition recursively", func(t *testing.T) {
		client := startedClient(t)
		writeEventTo(t, client, "/test")
		writeEventTo(t, client, "/test/nested")

		// The latest event of /test alone is 0, and the latest one of /test
		// together with the subjects below it is 1.
		assertPreconditionsOnSubject(t, client, []preconditionOnSubject{
			{precondition: eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "0", eventsourcingdb.PreconditionOptions{Recursive: false}), holds: true},
			{precondition: eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "1", eventsourcingdb.PreconditionOptions{Recursive: false}), holds: false},
			{precondition: eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "1", eventsourcingdb.PreconditionOptions{Recursive: true}), isRecursive: true, holds: true},
			{precondition: eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "0", eventsourcingdb.PreconditionOptions{Recursive: true}), isRecursive: true, holds: false},
		})
	})

	t.Run("supports the isEventQlQueryTrue precondition", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsEventQLQueryTruePrecondition("FROM e IN events PROJECT INTO COUNT() == 0"),
			},
		)

		assertCodeOfServer(t, err, "write events", eventsourcingdb.CodePreconditionViolated)
	})

	t.Run("reports an event that does not match its schema", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		err = client.RegisterEventSchema(
			ctx,
			"io.eventsourcingdb.test",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"value": map[string]any{
						"type": "number",
					},
				},
				"required":             []string{"value"},
				"additionalProperties": false,
			},
		)
		require.NoError(t, err)

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				{
					Source:  "https://www.eventsourcingdb.io",
					Subject: "/test",
					Type:    "io.eventsourcingdb.test",
					Data: map[string]any{
						"value": 23,
						"extra": true,
					},
				},
			},
			nil,
		)

		assertCodeOfServer(t, err, "write events", eventsourcingdb.CodeEventSchemaViolated)
		assert.ErrorContains(t, err, "additionalProperties 'extra' not allowed")
	})
}

// startedClient starts a database in a container, which it stops once the
// test is over, and returns a client of it.
func startedClient(t *testing.T) *eventsourcingdb.Client {
	t.Helper()

	imageTag, err := getImageTag()
	require.NoError(t, err)

	container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
	require.NoError(t, container.Start(t.Context()))

	// The context of the test ends before its cleanups run.
	t.Cleanup(func() {
		_ = container.Stop(context.Background())
	})

	client, err := container.GetClient(t.Context())
	require.NoError(t, err)

	return client
}

// writeEventTo writes an event to the given subject, without a precondition.
func writeEventTo(t *testing.T, client *eventsourcingdb.Client, subject string) {
	t.Helper()

	_, err := client.WriteEvents(t.Context(), []eventsourcingdb.EventCandidate{eventTo(subject)}, nil)
	require.NoError(t, err)
}

// eventTo returns an event candidate for the given subject.
func eventTo(subject string) eventsourcingdb.EventCandidate {
	return eventsourcingdb.EventCandidate{
		Source:  "https://www.eventsourcingdb.io",
		Subject: subject,
		Type:    "io.eventsourcingdb.test",
		Data:    map[string]int{"value": 23},
	}
}

// preconditionOnSubject is a precondition on a subject, whether it is
// recursive, and whether it holds.
type preconditionOnSubject struct {
	precondition eventsourcingdb.Precondition
	isRecursive  bool
	holds        bool
}

// assertPreconditionsOnSubject writes an event with each of the given
// preconditions in turn, and asserts that it is written if the precondition
// holds, and that the write fails as violated otherwise. A server without the
// v2 API can not check a recursive precondition, so there the write fails
// with ErrIncompatibleServer instead. The client must have sent a request
// before, so that it knows which API the server offers.
//
// The event goes to /written, which none of the preconditions looks at, so
// that writing it changes none of them, and so that the events of /written
// tell whether a write was written.
func assertPreconditionsOnSubject(t *testing.T, client *eventsourcingdb.Client, preconditions []preconditionOnSubject) {
	t.Helper()

	api := eventsourcingdb.APIOf(client)
	require.NotEmpty(t, api)

	writtenEvents := 0
	for index, precondition := range preconditions {
		t.Run(fmt.Sprintf("%d, recursive %t, holds %t, %s API", index, precondition.isRecursive, precondition.holds, api), func(t *testing.T) {
			_, err := client.WriteEvents(
				t.Context(),
				[]eventsourcingdb.EventCandidate{eventTo("/written")},
				[]eventsourcingdb.Precondition{precondition.precondition},
			)

			switch {
			case precondition.isRecursive && api == "v1":
				assert.ErrorIs(t, err, eventsourcingdb.ErrIncompatibleServer)
			case precondition.holds:
				assert.NoError(t, err)
				writtenEvents++
			default:
				assertCodeOfServer(t, err, "write events", eventsourcingdb.CodePreconditionViolated)
			}

			assert.Equal(t, writtenEvents, countEventsOf(t, client, "/written"))
		})
	}
}

// countEventsOf returns how many events the given subject has, without the
// subjects below it.
func countEventsOf(t *testing.T, client *eventsourcingdb.Client, subject string) int {
	t.Helper()

	count, err := countItems(client.ReadEvents(t.Context(), subject, eventsourcingdb.ReadEventsOptions{Recursive: false}), nil)
	require.NoError(t, err)

	return count
}
