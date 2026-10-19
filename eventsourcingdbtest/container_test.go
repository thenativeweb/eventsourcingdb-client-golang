package eventsourcingdbtest_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	dockercontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
)

func TestContainer(t *testing.T) {
	t.Run("starts, hands out a client, and stops", func(t *testing.T) {
		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		assert.False(t, container.IsRunning())

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.True(t, container.IsRunning())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.Ping(t.Context())
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
		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
			WithPort(4000).
			WithAPIToken("custom-token")

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.Equal(t, "custom-token", container.GetAPIToken())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.VerifyAPIToken(t.Context())
		assert.NoError(t, err)
	})

	t.Run("signs events with its signing key", func(t *testing.T) {
		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
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

		written, err := client.WriteEvents(t.Context(), []eventsourcingdb.EventCandidate{{
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

// Start replaces a container that runs, but in which the database does not
// become reachable within the startup timeout, up to 3 containers in total.
// If Docker fails, or if the container exits, it fails at once. Whenever an
// attempt fails, whatever the reason was, its container must not be left
// behind.
func TestContainerStart(t *testing.T) {
	imageTag, err := internal.GetImageTag()
	require.NoError(t, err)

	t.Run("waits for 30 seconds by default", func(t *testing.T) {
		container := eventsourcingdbtest.NewContainer()

		assert.Equal(t, 30*time.Second, eventsourcingdbtest.GetStartupTimeout(container))
	})

	t.Run("requires a startup timeout greater than zero", func(t *testing.T) {
		for _, startupTimeout := range []time.Duration{0, -time.Second} {
			attempts := replaceStart(t, startUnchanged)

			container := eventsourcingdbtest.NewContainer().
				WithImageTag(imageTag).
				WithStartupTimeout(startupTimeout)

			err := container.Start(t.Context())

			assert.EqualError(t, err, "startup timeout must be greater than zero")
			assert.Empty(t, attempts.IDs())
			assert.False(t, container.IsRunning())
		}
	})

	t.Run("does not count the time before Docker begins to create the container, such as pulling the image", func(t *testing.T) {
		const startupTimeout = 100 * time.Millisecond
		errStop := errors.New("stop")

		// The attempt ends before Docker begins to create a container, so
		// that how fast Docker is does not matter.
		var errBeforeCreate error
		attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
			// As if pulling the image took longer than the startup timeout.
			time.Sleep(3 * startupTimeout)
			errBeforeCreate = ctx.Err()

			return nil, errStop
		})

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
			WithStartupTimeout(startupTimeout)

		err := container.Start(t.Context())

		assert.NoError(t, errBeforeCreate)
		assert.EqualError(t, err, "failed to start the container: stop")
		assert.Len(t, attempts.IDs(), 1)
		assert.False(t, container.IsRunning())
	})

	t.Run("reads the port of the database anew for each ping", func(t *testing.T) {
		// A stale port, on which something else answers with a status code
		// of success, but not with 200 OK.
		var stalePings atomic.Int32
		stale := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			stalePings.Add(1)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer stale.Close()

		staleURL, err := url.Parse(stale.URL)
		require.NoError(t, err)
		stalePort, err := strconv.ParseUint(staleURL.Port(), 10, 16)
		require.NoError(t, err)

		var late *latePortContainer
		attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
			container, err := testcontainers.GenericContainer(ctx, request)
			if err != nil {
				return container, err
			}

			late = &latePortContainer{Container: container, stalePort: uint16(stalePort)}
			return late, nil
		})

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)

		err = container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.Len(t, attempts.IDs(), 1)
		assert.Greater(t, late.lookups, 6)
		assert.Positive(t, stalePings.Load())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.Ping(t.Context())
		assert.NoError(t, err)
	})

	t.Run("replaces a container in which the database does not become reachable", func(t *testing.T) {
		attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
			switch attempt {
			case 1:
				withoutNetwork(&request)
			case 2:
				listeningElsewhere(&request)
			default:
				// The container that replaces the others is ready before the
				// clock starts, so that how fast Docker starts it does not
				// matter.
				request.WaitingFor = wait.ForHTTP("/api/v1/ping").
					WithPort("3000/tcp").
					WithStartupTimeout(time.Minute)
				return startBeforeTheClock(ctx, request)
			}
			return testcontainers.GenericContainer(ctx, request)
		})

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
			WithSigningKey().
			WithStartupTimeout(time.Second)

		err := container.Start(t.Context())
		require.NoError(t, err)
		defer container.Stop(t.Context())

		assert.True(t, container.IsRunning())
		require.Len(t, attempts.IDs(), 3)
		assertDistinct(t, attempts.IDs())
		assertRemoved(t, attempts.IDs()[:2])

		// The container that replaced the others signs events, too, so it
		// got the signing key as well.
		verificationKey, err := container.GetVerificationKey()
		require.NoError(t, err)
		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		written, err := client.WriteEvents(t.Context(), []eventsourcingdb.EventCandidate{{
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

	t.Run("gives up after 3 containers in which the database does not become reachable", func(t *testing.T) {
		unreachables := []struct {
			name           string
			change         func(request *testcontainers.GenericContainerRequest)
			startupTimeout time.Duration
			message        string
		}{
			{
				name:           "as Docker does not publish its port",
				change:         withoutNetwork,
				startupTimeout: 2 * time.Second,
				message:        "failed to start the container, the database did not become reachable within 2s in any of 3 containers",
			},
			{
				name:           "as it does not listen on the port that Docker publishes",
				change:         listeningElsewhere,
				startupTimeout: 1500 * time.Millisecond,
				message:        "failed to start the container, the database did not become reachable within 1.5s in any of 3 containers",
			},
			{
				name:           "as Docker does not hand out the container in time",
				change:         nil,
				startupTimeout: time.Second,
				message:        "failed to start the container, the database did not become reachable within 1s in any of 3 containers",
			},
		}

		for _, unreachable := range unreachables {
			t.Run(unreachable.name, func(t *testing.T) {
				// A fourth attempt would work, so that Start does not try
				// forever if it does not stop.
				attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
					if attempt > 3 {
						return testcontainers.GenericContainer(ctx, request)
					}
					if unreachable.change == nil {
						return createdButNotHandedOut(ctx, request)
					}

					unreachable.change(&request)
					return testcontainers.GenericContainer(ctx, request)
				})

				container := eventsourcingdbtest.NewContainer().
					WithImageTag(imageTag).
					WithStartupTimeout(unreachable.startupTimeout)

				startedAt := time.Now()
				err := container.Start(t.Context())
				defer container.Stop(t.Context())
				elapsed := time.Since(startedAt)

				assert.EqualError(t, err, unreachable.message)
				require.Len(t, attempts.IDs(), 3)
				assertDistinct(t, attempts.IDs())
				assert.False(t, container.IsRunning())
				assertRemoved(t, attempts.IDs())
				assertTerminated(t, attempts)

				// Each container got the startup timeout that was set, and not
				// the default.
				assert.GreaterOrEqual(t, elapsed, 3*unreachable.startupTimeout)
				assert.Less(t, elapsed, 3*eventsourcingdbtest.GetStartupTimeout(eventsourcingdbtest.NewContainer()))
			})
		}
	})

	t.Run("fails at once if the container exits, and names the end of its log", func(t *testing.T) {
		t.Run("with an invalid flag", func(t *testing.T) {
			attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
				request.Cmd = append(request.Cmd, "--does-not-exist")
				return testcontainers.GenericContainer(ctx, request)
			})

			container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)

			err := container.Start(t.Context())
			defer container.Stop(t.Context())

			require.Error(t, err)
			prefix := "failed to start the container, it exited before the database became reachable: "
			require.True(t, strings.HasPrefix(err.Error(), prefix), err.Error())

			// The database writes the usage after the error, which is more
			// than 20 lines long.
			logTail := strings.TrimPrefix(err.Error(), prefix)
			assert.Len(t, strings.Split(logTail, "\n"), 20)
			assert.Contains(t, logTail, "--http-port")

			assert.Len(t, attempts.IDs(), 1)
			assert.False(t, container.IsRunning())
			assertRemoved(t, attempts.IDs())
			assertTerminated(t, attempts)
		})

		t.Run("with exactly the last 20 lines of its log", func(t *testing.T) {
			attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
				request.Entrypoint = []string{"sh", "-c", `i=1; while [ $i -le 30 ]; do echo "line $i"; i=$((i+1)); done; exit 1`}
				request.Cmd = nil
				return testcontainers.GenericContainer(ctx, request)
			})

			container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)

			err := container.Start(t.Context())
			defer container.Stop(t.Context())

			lines := []string{}
			for line := 11; line <= 30; line++ {
				lines = append(lines, fmt.Sprintf("line %d", line))
			}
			assert.EqualError(t, err, "failed to start the container, it exited before the database became reachable: "+strings.Join(lines, "\n"))
			assert.Len(t, attempts.IDs(), 1)
			assert.False(t, container.IsRunning())
			assertRemoved(t, attempts.IDs())
			assertTerminated(t, attempts)
		})
	})

	t.Run("fails at once if Docker fails, with the error of Docker", func(t *testing.T) {
		attempts := replaceStart(t, startUnchanged)

		container := eventsourcingdbtest.NewContainer().WithImageTag("does-not-exist")

		err := container.Start(t.Context())
		defer container.Stop(t.Context())

		require.Len(t, attempts.IDs(), 1)
		require.Error(t, attempts.Errs()[0])
		assert.EqualError(t, err, "failed to start the container: "+attempts.Errs()[0].Error())
		assert.ErrorIs(t, err, attempts.Errs()[0])
		assert.False(t, container.IsRunning())
		assertRemoved(t, attempts.IDs())
	})

	t.Run("fails at once if Docker fails while it waits for the database", func(t *testing.T) {
		errDocker := errors.New("Docker failed")

		attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
			container, err := startBeforeTheClock(ctx, request)
			if err != nil {
				return container, err
			}
			return failingStateContainer{Container: container, err: errDocker}, nil
		})

		container := eventsourcingdbtest.NewContainer().
			WithImageTag(imageTag).
			WithStartupTimeout(time.Second)

		err := container.Start(t.Context())
		defer container.Stop(t.Context())

		assert.EqualError(t, err, "failed to start the container: Docker failed")
		assert.ErrorIs(t, err, errDocker)
		assert.Len(t, attempts.IDs(), 1)
		assert.False(t, container.IsRunning())
		assertRemoved(t, attempts.IDs())
		assertTerminated(t, attempts)
	})

	t.Run("removes the container once the context of Start ends", func(t *testing.T) {
		ends := []struct {
			name  string
			start func(ctx context.Context, cancel context.CancelFunc, request testcontainers.GenericContainerRequest) (testcontainers.Container, error)
		}{
			{
				name: "once Docker created it, but before it was handed out",
				start: func(ctx context.Context, cancel context.CancelFunc, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
					_, _ = testcontainers.GenericContainer(ctx, request)
					cancel()
					return nil, context.Canceled
				},
			},
			{
				name: "while it waits for the database",
				start: func(ctx context.Context, cancel context.CancelFunc, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
					withoutNetwork(&request)
					container, err := testcontainers.GenericContainer(ctx, request)
					cancel()
					return container, err
				},
			},
		}

		for _, end := range ends {
			t.Run(end.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
					return end.start(ctx, cancel, request)
				})

				container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)

				err := container.Start(ctx)
				defer container.Stop(t.Context())

				assert.EqualError(t, err, "failed to start the container: context canceled")
				assert.ErrorIs(t, err, context.Canceled)
				require.Len(t, attempts.IDs(), 1)
				assert.False(t, container.IsRunning())
				assertRemoved(t, attempts.IDs())
				assertTerminated(t, attempts)
			})
		}

		t.Run("while Docker creates it", func(t *testing.T) {
			// If the context ended the call that creates the container, Docker
			// could still create the container after Start looked for it, so
			// the call must not be cut short.
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var cutShort atomic.Bool
			attempts := replaceStart(t, func(attemptCtx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
				request.LifecycleHooks = append(request.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
					PreCreates: []testcontainers.ContainerRequestHook{
						func(context.Context, testcontainers.ContainerRequest) error {
							cancel()
							cutShort.Store(attemptCtx.Err() != nil)
							return nil
						},
					},
				})
				return testcontainers.GenericContainer(attemptCtx, request)
			})

			container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)

			err := container.Start(ctx)
			defer container.Stop(t.Context())

			assert.False(t, cutShort.Load(), "the context cut creating the container short")
			assert.EqualError(t, err, "failed to start the container: context canceled")
			require.Len(t, attempts.IDs(), 1)
			assert.False(t, container.IsRunning())
			assertRemoved(t, attempts.IDs())
			assertTerminated(t, attempts)
		})

		t.Run("while Docker hangs creating it, for the grace period at most", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			errHung := errors.New("Docker hung")

			var attemptID string
			var canceledAt time.Time
			done := make(chan struct{})
			eventsourcingdbtest.SetStartContainer(t, func(attemptCtx context.Context, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
				defer close(done)
				attemptID = request.Labels[eventsourcingdbtest.StartLabel]

				// The context of Start ends while Docker creates the
				// container. Docker hangs beyond the grace period, and creates
				// the container only once the attempt stopped waiting for it.
				late := request
				late.LifecycleHooks = nil
				request.LifecycleHooks = append(request.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
					PreCreates: []testcontainers.ContainerRequestHook{
						func(context.Context, testcontainers.ContainerRequest) error {
							canceledAt = time.Now()
							cancel()
							time.Sleep(eventsourcingdbtest.CreateGracePeriod + 500*time.Millisecond)
							return errHung
						},
					},
				})

				_, err := testcontainers.GenericContainer(attemptCtx, request)
				_, _ = testcontainers.GenericContainer(context.Background(), late)
				return nil, err
			})
			t.Cleanup(func() {
				<-done
				err := eventsourcingdbtest.RemoveContainers(context.Background(), attemptID)
				assert.NoError(t, err)
			})

			container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)

			err := container.Start(ctx)
			returnedAt := time.Now()

			assert.EqualError(t, err, "failed to start the container: context canceled")
			assert.False(t, container.IsRunning())

			// Once Docker created the container late, it is removed in the
			// background.
			<-done
			waited := returnedAt.Sub(canceledAt)
			assert.GreaterOrEqual(t, waited, eventsourcingdbtest.CreateGracePeriod)
			assert.Less(t, waited, eventsourcingdbtest.CreateGracePeriod+time.Second)

			dockerClient, err := testcontainers.NewDockerClientWithOpts(context.Background())
			require.NoError(t, err)
			defer dockerClient.Close()

			assert.Eventually(t, func() bool {
				containers, err := dockerClient.ContainerList(context.Background(), client.ContainerListOptions{
					All:     true,
					Filters: make(client.Filters).Add("label", eventsourcingdbtest.StartLabel+"="+attemptID),
				})
				return err == nil && len(containers.Items) == 0
			}, eventsourcingdbtest.CreateGracePeriod, 100*time.Millisecond, "the container that Docker created late was left behind")
		})

		t.Run("at any time, within a deadline", func(t *testing.T) {
			for _, deadline := range []time.Duration{25, 50, 75, 100, 150, 200, 300, 400} {
				ctx, cancel := context.WithTimeout(t.Context(), deadline*time.Millisecond)

				attempts := replaceStart(t, startUnchanged)
				container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)

				err := container.Start(ctx)
				cancel()

				// A start that is faster than the deadline is fine, too.
				if err == nil {
					err = container.Stop(t.Context())
					require.NoError(t, err)
				} else {
					assert.EqualError(t, err, "failed to start the container: context deadline exceeded", "deadline %s", deadline*time.Millisecond)
				}
				assert.False(t, container.IsRunning())
				assertRemoved(t, attempts.IDs())
			}
		})
	})

	t.Run("returns at once if it can not remove a container, and names that after the error of the attempt", func(t *testing.T) {
		failures := []struct {
			name           string
			imageTag       string
			change         func(request *testcontainers.GenericContainerRequest)
			startupTimeout time.Duration
			message        func(attempts *attempts) string
		}{
			{
				name:           "as the database does not become reachable",
				imageTag:       imageTag,
				change:         withoutNetwork,
				startupTimeout: time.Second,
				message: func(*attempts) string {
					return "failed to start the container, the database did not become reachable within 1s"
				},
			},
			{
				name:           "as Docker fails",
				imageTag:       "does-not-exist",
				change:         func(*testcontainers.GenericContainerRequest) {},
				startupTimeout: time.Second,
				message: func(attempts *attempts) string {
					return "failed to start the container: " + attempts.Errs()[0].Error()
				},
			},
		}

		for _, failure := range failures {
			t.Run(failure.name, func(t *testing.T) {
				errRemove := errors.New("failed to remove container")
				eventsourcingdbtest.SetRemoveContainers(t, func(ctx context.Context, attemptID string) error {
					err := eventsourcingdbtest.RemoveContainers(ctx, attemptID)
					require.NoError(t, err)
					return errRemove
				})

				attempts := replaceStart(t, func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
					failure.change(&request)
					return startBeforeTheClock(ctx, request)
				})

				container := eventsourcingdbtest.NewContainer().
					WithImageTag(failure.imageTag).
					WithStartupTimeout(failure.startupTimeout)

				err := container.Start(t.Context())
				defer container.Stop(t.Context())

				require.Len(t, attempts.IDs(), 1)
				assert.EqualError(t, err, failure.message(attempts)+"\nfailed to remove the container: failed to remove container")
				assert.ErrorIs(t, err, errRemove)
				assert.False(t, container.IsRunning())
			})
		}
	})
}

// attempts records the id of each attempt of Start to start a container, and
// what creating and starting the container returned. Start may stop waiting
// for that, so the record is safe for concurrent use.
type attempts struct {
	mu         sync.Mutex
	ids        []string
	containers []*trackedContainer
	errs       []error
}

func (a *attempts) IDs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return slices.Clone(a.ids)
}

func (a *attempts) Errs() []error {
	a.mu.Lock()
	defer a.mu.Unlock()

	return slices.Clone(a.errs)
}

func (a *attempts) Containers() []*trackedContainer {
	a.mu.Lock()
	defer a.mu.Unlock()

	return slices.Clone(a.containers)
}

// replaceStart makes Start call start for each attempt to create and start a
// container, with the number of the attempt, until the test t ends. start can
// change the request before it starts a container for it. Every container of
// an attempt is removed once the test ends, so that none is left behind even
// if Start fails to remove it.
func replaceStart(
	t *testing.T,
	start func(ctx context.Context, attempt int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error),
) *attempts {
	t.Helper()

	recorded := &attempts{}
	eventsourcingdbtest.SetStartContainer(t, func(ctx context.Context, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
		attemptID := request.Labels[eventsourcingdbtest.StartLabel]
		assert.NotEmpty(t, attemptID)
		t.Cleanup(func() {
			err := eventsourcingdbtest.RemoveContainers(context.Background(), attemptID)
			assert.NoError(t, err)
		})

		recorded.mu.Lock()
		recorded.ids = append(recorded.ids, attemptID)
		attempt := len(recorded.ids)
		recorded.mu.Unlock()

		container, err := start(ctx, attempt, request)

		var tracked *trackedContainer
		if container != nil {
			tracked = &trackedContainer{Container: container}
		}

		recorded.mu.Lock()
		recorded.errs = append(recorded.errs, err)
		recorded.containers = append(recorded.containers, tracked)
		recorded.mu.Unlock()

		if tracked == nil {
			return nil, err
		}
		return tracked, err
	})

	return recorded
}

// startBeforeTheClock creates and starts a container without the hooks of
// Start, so that the clock of the attempt starts only once the call returned,
// and how fast Docker creates and starts the container does not count.
func startBeforeTheClock(ctx context.Context, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
	request.LifecycleHooks = nil
	return testcontainers.GenericContainer(ctx, request)
}

// startUnchanged creates and starts a container as Start does.
func startUnchanged(ctx context.Context, _ int, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
	return testcontainers.GenericContainer(ctx, request)
}

// createdButNotHandedOut creates and starts a container, but behaves as if
// Docker had not answered before the context ended, so that it does not hand
// out the container.
func createdButNotHandedOut(ctx context.Context, request testcontainers.GenericContainerRequest) (testcontainers.Container, error) {
	_, _ = testcontainers.GenericContainer(ctx, request)
	<-ctx.Done()
	return nil, ctx.Err()
}

// withoutNetwork changes a request, so that the container gets no network.
// Docker then lists the port of the database, but can not publish it, as
// when Docker Desktop fails to publish it.
func withoutNetwork(request *testcontainers.GenericContainerRequest) {
	request.Networks = []string{"none"}
}

// listeningElsewhere changes a request, so that the database listens on
// another port than the one that Docker publishes. The container runs, but a
// ping never reaches the database.
func listeningElsewhere(request *testcontainers.GenericContainerRequest) {
	for i, arg := range request.Cmd {
		if arg == "--http-port" {
			request.Cmd[i+1] = "4000"
		}
	}
}

// latePortContainer is a container whose port Docker publishes only late:
// the first lookups of the port find none, the next ones a stale port on
// which something else answers, and only the ones after that find the port
// of the database.
type latePortContainer struct {
	testcontainers.Container
	stalePort uint16
	lookups   int
}

func (c *latePortContainer) MappedPort(ctx context.Context, port string) (network.Port, error) {
	c.lookups++

	switch {
	case c.lookups <= 3:
		return network.Port{}, errdefs.ErrNotFound.WithMessage(fmt.Sprintf("port %q not found", port))
	case c.lookups <= 6:
		stalePort, _ := network.PortFrom(c.stalePort, network.TCP)
		return stalePort, nil
	default:
		return c.Container.MappedPort(ctx, port)
	}
}

// failingStateContainer is a container for which Docker fails to tell its
// state.
type failingStateContainer struct {
	testcontainers.Container
	err error
}

func (c failingStateContainer) State(context.Context) (*dockercontainer.State, error) {
	return nil, c.err
}

// trackedContainer is a container that counts how often it was terminated.
type trackedContainer struct {
	testcontainers.Container
	terminations atomic.Int32
}

func (c *trackedContainer) Terminate(ctx context.Context, options ...testcontainers.TerminateOption) error {
	c.terminations.Add(1)
	return c.Container.Terminate(ctx, options...)
}

// assertTerminated asserts that each container that was handed out to Start
// was terminated once, so that Testcontainers released what it holds for it.
// That may happen in the background, within the grace period, if Start
// stopped waiting for Docker to create the container.
func assertTerminated(t *testing.T, attempts *attempts) {
	t.Helper()

	assert.Eventually(t, func() bool {
		for _, container := range attempts.Containers() {
			if container != nil && container.terminations.Load() != 1 {
				return false
			}
		}
		return true
	}, 2*eventsourcingdbtest.CreateGracePeriod, 100*time.Millisecond, "a container was not terminated once")
}

// assertDistinct asserts that each attempt has an id of its own.
func assertDistinct(t *testing.T, attemptIDs []string) {
	t.Helper()

	distinct := map[string]bool{}
	for _, attemptID := range attemptIDs {
		distinct[attemptID] = true
	}
	assert.Len(t, distinct, len(attemptIDs))
}

// assertRemoved asserts that Docker no longer knows any container of the
// given attempts. If Start stopped waiting for Docker to create a container,
// it removes the container in the background, within the grace period.
func assertRemoved(t *testing.T, attemptIDs []string) {
	t.Helper()

	dockerClient, err := testcontainers.NewDockerClientWithOpts(context.Background())
	require.NoError(t, err)
	defer dockerClient.Close()

	assert.Eventually(t, func() bool {
		for _, attemptID := range attemptIDs {
			containers, err := dockerClient.ContainerList(context.Background(), client.ContainerListOptions{
				All:     true,
				Filters: make(client.Filters).Add("label", eventsourcingdbtest.StartLabel+"="+attemptID),
			})
			if err != nil || len(containers.Items) > 0 {
				return false
			}
		}
		return true
	}, 2*eventsourcingdbtest.CreateGracePeriod, 100*time.Millisecond, "a container of the attempts was left behind")
}
