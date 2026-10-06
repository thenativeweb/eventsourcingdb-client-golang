package eventsourcingdbtest

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

// SetStartContainer replaces how Start creates and starts a container until
// the test t ends, so that a test can decide what happens on each attempt.
func SetStartContainer(
	t *testing.T,
	start func(ctx context.Context, request testcontainers.GenericContainerRequest) (testcontainers.Container, error),
) {
	t.Helper()

	original := startContainer
	startContainer = start

	t.Cleanup(func() {
		startContainer = original
	})
}
