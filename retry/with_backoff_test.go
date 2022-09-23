package retry_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
)

func TestWithBackoff(t *testing.T) {
	t.Run("returns immediately if no error occurs.", func(t *testing.T) {
		count := 0
		maxTries := 3

		err := retry.WithBackoff(func() error {
			count += 1
			return nil
		}, maxTries, context.Background())

		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("returns the error if an error occurs.", func(t *testing.T) {
		count := 0
		maxTries := 3

		err := retry.WithBackoff(func() error {
			count += 1
			return errors.New("something went wrong")
		}, maxTries, context.Background())

		assert.Error(t, err)
		assert.Equal(t, maxTries, count)
	})

	t.Run("returns when no error occurs any more.", func(t *testing.T) {
		count := 0
		maxTries := 5
		successfulTry := 3

		err := retry.WithBackoff(func() error {
			count += 1
			if count != successfulTry {
				return errors.New("something went wrong")
			}
			return nil
		}, maxTries, context.Background())

		assert.NoError(t, err)
		assert.Equal(t, successfulTry, count)
	})

	t.Run("returns an error immediately when the context is done.", func(t *testing.T) {
		count := 0
		maxTries := 5
		cancellingTry := 3

		context, cancel := context.WithCancel(context.Background())

		err := retry.WithBackoff(func() error {
			count += 1
			if count == cancellingTry {
				cancel()
			}
			return errors.New("something went wrong")
		}, maxTries, context)

		assert.Error(t, err)
		assert.Equal(t, cancellingTry, count)
	})
}
