package eventsourcingdb_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"testing"
)

func TestReadSubjects(t *testing.T) {
	t.Run("returns a channel containing an error when trying to read from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()

		readSubjectResults := client.ReadSubjects(context.Background())

		errorResult := <-readSubjectResults
		assert.True(t, errorResult.IsError())
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		readSubjectResults := client.ReadSubjects(context.Background())

		for result := range readSubjectResults {
			_, err := result.GetData()
			assert.NoError(t, err)
		}
	})

	t.Run("closes the channel when no more stream names exist.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		readSubjectResults := client.ReadSubjects(context.Background())

		rootSubjectResult := <-readSubjectResults
		rootSubject, err := rootSubjectResult.GetData()

		assert.NoError(t, err)
		assert.Equal(t, "/", rootSubject)

		_, ok := <-readSubjectResults
		assert.False(t, ok)
	})

	t.Run("reads all stream names starting from /.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]event.Candidate{
			event.NewCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
		})

		assert.NoError(t, err)

		readSubjectResults := client.ReadSubjects(context.Background())
		subjects := make([]string, 0, 2)

		for result := range readSubjectResults {
			data, err := result.GetData()
			assert.NoError(t, err)

			subjects = append(subjects, data)
		}

		assert.Equal(t, []string{"/", subject}, subjects)
	})

	t.Run("reads stream names starting from the given base stream name.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		subject := "/foobar/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]event.Candidate{
			event.NewCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
		})

		assert.NoError(t, err)

		readSubjectResults := client.ReadSubjects(context.Background(), eventsourcingdb.BaseSubject("/foobar"))
		subjects := make([]string, 0, 2)

		for result := range readSubjectResults {
			data, err := result.GetData()
			assert.NoError(t, err)

			subjects = append(subjects, data)
		}

		assert.Equal(t, []string{"/foobar", subject}, subjects)
	})

	t.Run("closes the result channel when the given context is canceled.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		readSubjectResults := client.ReadSubjects(ctx)

		cancelledResult := <-readSubjectResults
		_, err := cancelledResult.GetData()
		assert.Error(t, err)
		assert.True(t, customErrors.IsContextCanceledError(err))

		superfluousResult, ok := <-readSubjectResults
		assert.False(t, ok, fmt.Sprintf("channel did not close %+v", superfluousResult))
	})

	t.Run("returns an error when the base subject is malformed.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		results := client.ReadSubjects(context.Background(), eventsourcingdb.BaseSubject("schkibididopdop"))
		result := <-results

		_, err := result.GetData()
		assert.ErrorContains(t, err, "malformed event subject")
	})
}
