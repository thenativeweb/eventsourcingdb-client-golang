package retry_test

import (
	"errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryError(t *testing.T) {
	t.Run("has a default error message.", func(t *testing.T) {
		err := retry.NewRetryError()

		assert.Error(t, err)
		assert.Equal(t, "retries exceeded", err.Error())
	})

	t.Run("handles nested errors.", func(t *testing.T) {
		err := retry.NewRetryError()
		err.AppendError(errors.New("something went wrong"))

		assert.Error(t, err)
		assert.Equal(t, "retries exceeded\n\ttry 1: something went wrong", err.Error())
	})

	t.Run("handles nested errors in correct order.", func(t *testing.T) {
		err := retry.NewRetryError()
		err.AppendError(errors.New("#1"))
		err.AppendError(errors.New("#2"))
		err.AppendError(errors.New("#3"))

		assert.Error(t, err)
		assert.Equal(t, "retries exceeded\n\ttry 1: #1\n\ttry 2: #2\n\ttry 3: #3", err.Error())
	})
}
