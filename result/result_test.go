package result_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/result"
	"testing"
)

func TestIsError(t *testing.T) {
	t.Run("returns true for an error result.", func(t *testing.T) {
		resultWithError := result.NewResultWithError[any](assert.AnError)

		isError := resultWithError.IsError()

		assert.True(t, isError)
	})

	t.Run("returns false for an data result", func(t *testing.T) {
		resultWithData := result.NewResultWithData("some data")

		isError := resultWithData.IsError()

		assert.False(t, isError)
	})
}

func TestIsData(t *testing.T) {
	t.Run("returns false for an error result.", func(t *testing.T) {
		resultWithError := result.NewResultWithError[any](assert.AnError)

		isData := resultWithError.IsData()

		assert.False(t, isData)
	})

	t.Run("returns true for an data result", func(t *testing.T) {
		resultWithData := result.NewResultWithData("some data")

		isData := resultWithData.IsData()

		assert.True(t, isData)
	})
}

func TestGetData(t *testing.T) {
	t.Run("returns an error and the zero value for data part for an error result.", func(t *testing.T) {
		resultWithError := result.NewResultWithError[string](assert.AnError)

		data, err := resultWithError.GetData()

		assert.Error(t, err)
		assert.Empty(t, data)
	})

	t.Run("returns the given data and no error for an data result", func(t *testing.T) {
		resultWithData := result.NewResultWithData("some data")

		data, err := resultWithData.GetData()

		assert.NoError(t, err)
		assert.Equal(t, "some data", data)
	})
}
