package eventsourcingdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
)

func TestPing(t *testing.T) {
	client := eventsourcingdb.NewClient(baseUrl)
	err := client.Ping()

	assert.NoError(t, err)
}
