package eventsourcingdbtest

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// StartLabel is the label that Start gives each container it starts, with an
// id of the attempt as its value.
const StartLabel = startLabel

// CreateGracePeriod is how long an attempt waits at most for Docker to finish
// creating a container once its context ended.
const CreateGracePeriod = createGracePeriod

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

// SetRemoveContainers replaces how Start removes the containers of an attempt
// until the test t ends, e.g. to fail as if removing them had failed.
func SetRemoveContainers(t *testing.T, remove func(ctx context.Context, attemptID string) error) {
	t.Helper()

	original := removeContainers
	removeContainers = remove

	t.Cleanup(func() {
		removeContainers = original
	})
}

// RemoveContainers removes the containers of an attempt, as Start does.
func RemoveContainers(ctx context.Context, attemptID string) error {
	return removeContainersOfAttempt(ctx, attemptID)
}

// GetStartupTimeout returns the startup timeout of a container.
func GetStartupTimeout(c *Container) time.Duration {
	return c.startupTimeout
}
