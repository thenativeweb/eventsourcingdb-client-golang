package eventsourcingdbtest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestContainer(t *testing.T) {
	t.Run("starts, hands out a client, and stops", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)
		assert.False(t, container.IsRunning())

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.True(t, container.IsRunning())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.Ping()
		assert.NoError(t, err)

		host, err := container.GetHost(t.Context())
		require.NoError(t, err)
		port, err := container.GetMappedPort(t.Context())
		require.NoError(t, err)
		baseURL, err := container.GetBaseURL(t.Context())
		require.NoError(t, err)

		assert.Equal(t, fmt.Sprintf("http://%s:%d", host, port), baseURL.String())
		assert.Equal(t, "secret", container.GetAPIToken())

		err = container.Stop(t.Context())
		require.NoError(t, err)
		assert.False(t, container.IsRunning())

		// Stopping a container that is not running is fine.
		err = container.Stop(t.Context())
		assert.NoError(t, err)
	})

	t.Run("uses a custom API token and port", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageVersion).
			WithPort(4000).
			WithAPIToken("custom-token")

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.Equal(t, "custom-token", container.GetAPIToken())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.VerifyAPIToken()
		assert.NoError(t, err)
	})

	t.Run("signs events with its signing key", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageVersion).
			WithSigningKey()

		signingKey, err := container.GetSigningKey()
		require.NoError(t, err)
		assert.NotNil(t, signingKey)

		verificationKey, err := container.GetVerificationKey()
		require.NoError(t, err)

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		written, err := client.WriteEvents([]eventsourcingdb.EventCandidate{{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data:    map[string]any{"value": 23},
		}}, nil)
		require.NoError(t, err)
		require.Len(t, written, 1)

		err = written[0].VerifySignature(*verificationKey)
		assert.NoError(t, err)
	})

	t.Run("fails to start with an image that does not exist", func(t *testing.T) {
		container := eventsourcingdbtest.NewContainer().WithImageTag("does-not-exist")

		err := container.Start(t.Context())
		assert.Error(t, err)
		assert.False(t, container.IsRunning())
	})

	t.Run("reports what it does not have", func(t *testing.T) {
		container := eventsourcingdbtest.NewContainer()

		_, err := container.GetHost(t.Context())
		assert.Error(t, err)
		_, err = container.GetMappedPort(t.Context())
		assert.Error(t, err)
		_, err = container.GetBaseURL(t.Context())
		assert.Error(t, err)
		_, err = container.GetClient(t.Context())
		assert.Error(t, err)

		_, err = container.GetSigningKey()
		assert.Error(t, err)
		_, err = container.GetVerificationKey()
		assert.Error(t, err)
	})
}

// Docker Desktop sometimes fails to publish the port of a container on the
// host, because something on the host took that port in the meantime. The
// container keeps running, but the database in it can never be reached, so
// Start replaces such a container by a new one. A container that failed to
// start must never be left behind, whatever the reason was.
func TestContainerStart(t *testing.T) {
	imageVersion, err := internal.GetImageVersionFromDockerfile()
	require.NoError(t, err)

	t.Run("replaces a container whose port Docker did not publish", func(t *testing.T) {
		attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
			switch attempt {
			case 1:
				withoutNetwork(&request)
			case 2:
				withAnotherPortPublished(&request)
			}
			return testcontainers.GenericContainer(ctx, request)
		})

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageVersion).
			WithSigningKey()

		err := container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.True(t, container.IsRunning())
		require.Len(t, attempts.containers, 3)
		assertRemoved(t, attempts.containers[:2])

		// The container that replaced the others signs events, too, so it
		// got the signing key as well.
		verificationKey, err := container.GetVerificationKey()
		require.NoError(t, err)
		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		written, err := client.WriteEvents([]eventsourcingdb.EventCandidate{{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data:    map[string]any{"value": 23},
		}}, nil)
		require.NoError(t, err)
		require.Len(t, written, 1)

		err = written[0].VerifySignature(*verificationKey)
		assert.NoError(t, err)
	})

	t.Run("gives up after three attempts, and says why", func(t *testing.T) {
		// A fourth attempt would work, so that Start does not try forever if
		// it does not stop.
		attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
			if attempt <= 3 {
				withAnotherPortPublished(&request)
			}
			return testcontainers.GenericContainer(ctx, request)
		})

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)

		err := container.Start(t.Context())
		defer container.Stop(t.Context())

		assert.ErrorContains(t, err, "failed to start container, Docker did not publish port 3000/tcp in 3 attempts: start container: ")
		require.Len(t, attempts.containers, 3)
		assert.ErrorIs(t, err, attempts.errs[2])
		assert.False(t, container.IsRunning())
		assertRemoved(t, attempts.containers)
	})

	t.Run("returns any other failure at once, as it is, and terminates the container", func(t *testing.T) {
		failures := []struct {
			name   string
			change func(request *testcontainers.GenericContainerRequest)
		}{
			{name: "the database never gets ready", change: neverReady},
			{name: "the container exits", change: exiting},
		}

		for _, failure := range failures {
			t.Run(failure.name, func(t *testing.T) {
				attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
					failure.change(&request)
					return testcontainers.GenericContainer(ctx, request)
				})

				container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)

				err := container.Start(t.Context())
				defer container.Stop(t.Context())

				require.Len(t, attempts.containers, 1)
				require.Error(t, err)
				assert.Equal(t, attempts.errs[0], err)
				assert.False(t, container.IsRunning())
				assertRemoved(t, attempts.containers)
			})
		}
	})

	t.Run("makes no other attempt once the context has ended", func(t *testing.T) {
		t.Run("while the database starts", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
				withAnotherPortPublished(&request)
				request.WaitingFor = contextEndingWait{cancel: cancel}
				return testcontainers.GenericContainer(ctx, request)
			})

			container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)

			err := container.Start(ctx)
			defer container.Stop(t.Context())

			assert.ErrorIs(t, err, context.Canceled)
			require.Len(t, attempts.containers, 1)
			assert.False(t, container.IsRunning())
			assertRemoved(t, attempts.containers)
		})

		t.Run("while the container whose port Docker did not publish is terminated", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
				withAnotherPortPublished(&request)
				container, err := testcontainers.GenericContainer(ctx, request)
				return terminating(container, func() error {
					cancel()
					return nil
				}), err
			})

			container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)

			err := container.Start(ctx)
			defer container.Stop(t.Context())

			assert.ErrorContains(t, err, "failed to start container, Docker did not publish port 3000/tcp, and the context ended before another attempt: ")
			assert.ErrorIs(t, err, context.Canceled)
			require.Len(t, attempts.containers, 1)
			assert.ErrorIs(t, err, attempts.errs[0])
			assert.False(t, container.IsRunning())
			assertRemoved(t, attempts.containers)
		})
	})

	t.Run("returns at once if it can not terminate a container", func(t *testing.T) {
		errTerminate := errors.New("failed to remove container")

		attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
			withAnotherPortPublished(&request)
			container, err := testcontainers.GenericContainer(ctx, request)
			return terminating(container, func() error {
				return errTerminate
			}), err
		})

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)

		err := container.Start(t.Context())
		defer container.Stop(t.Context())

		assert.ErrorContains(t, err, ", and failed to terminate container: ")
		require.Len(t, attempts.containers, 1)
		assert.ErrorIs(t, err, attempts.errs[0])
		assert.ErrorIs(t, err, errTerminate)
		assert.False(t, container.IsRunning())
	})
}

// attempts records what each attempt of Start to start a container returned.
type attempts struct {
	containers []testcontainers.Container
	errs       []error
}

// replaceStart makes Start call start for each attempt to create and start a
// container, with the number of the attempt, until the test t ends. start can
// change the request before it starts a container for it.
func replaceStart(
	t *testing.T,
	start func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error),
) *attempts {
	t.Helper()

	recorded := &attempts{}
	eventsourcingdbtest.SetStartContainer(t, func(ctx context.Context, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
		container, err := start(ctx, len(recorded.containers)+1, request)
		recorded.containers = append(recorded.containers, container)
		recorded.errs = append(recorded.errs, err)
		return container, err
	})

	return recorded
}

// withoutNetwork changes a request, so that the container gets no network.
// Docker then lists the port of the database, but can not publish it, as
// when Docker Desktop fails to publish it. Waiting for the database gives up
// after a second, as the port never turns up.
func withoutNetwork(request *testcontainers.GenericContainerRequest) {
	request.Networks = []string{"none"}
	request.WaitingFor = waitBriefly()
}

// withAnotherPortPublished changes a request, so that Docker publishes another
// port of the image, but not the one of the database. Waiting for the
// database gives up after a second, as the port never turns up.
func withAnotherPortPublished(request *testcontainers.GenericContainerRequest) {
	request.ExposedPorts = []string{"4000/tcp"}
	request.WaitingFor = waitBriefly()
}

// neverReady changes a request, so that the database never counts as ready,
// although Docker publishes its port. Waiting for it gives up after a second.
func neverReady(request *testcontainers.GenericContainerRequest) {
	request.WaitingFor = waitBriefly().WithStatusCodeMatcher(func(int) bool {
		return false
	})
}

// exiting changes a request, so that the database exits as soon as it starts.
func exiting(request *testcontainers.GenericContainerRequest) {
	request.Cmd = []string{"does-not-exist"}
}

// waitBriefly waits for the database as Start does, but for one second only.
func waitBriefly() *wait.HTTPStrategy {
	return wait.ForHTTP("/api/v1/ping").
		WithPort("3000/tcp").
		WithStartupTimeout(time.Second)
}

// contextEndingWait is a wait strategy that ends the context of Start, and
// returns once it has ended.
type contextEndingWait struct {
	cancel context.CancelFunc
}

func (s contextEndingWait) WaitUntilReady(ctx context.Context, _ wait.StrategyTarget) error {
	s.cancel()
	<-ctx.Done()
	return ctx.Err()
}

// terminating wraps a container, if there is one, so that it calls terminated
// once it has been terminated, e.g. to fail as if terminating it had failed.
func terminating(container testcontainers.Container, terminated func() error) testcontainers.Container {
	if container == nil {
		return nil
	}

	return terminatingContainer{Container: container, terminated: terminated}
}

type terminatingContainer struct {
	testcontainers.Container
	terminated func() error
}

func (c terminatingContainer) Terminate(ctx context.Context, options ...testcontainers.TerminateOption) error {
	err := c.Container.Terminate(ctx, options...)
	if err != nil {
		return err
	}

	return c.terminated()
}

// assertRemoved asserts that Docker no longer knows any of the containers.
func assertRemoved(t *testing.T, containers []testcontainers.Container) {
	t.Helper()

	for _, container := range containers {
		_, err := container.Inspect(context.Background())
		assert.ErrorContains(t, err, "No such container", "the container was left behind")
	}
}
