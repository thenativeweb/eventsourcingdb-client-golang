package eventsourcingdbtest

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/errdefs"
	dockercontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// maxStartAttempts is how many containers Start starts at most, as it
// replaces each container in which the database does not become reachable
// within the startup timeout.
const maxStartAttempts = 3

// defaultStartupTimeout is how long each attempt of Start waits for the
// database to become reachable, unless WithStartupTimeout sets another time.
const defaultStartupTimeout = 30 * time.Second

// startLabel is the label that Start gives each container it starts, with an
// id of the attempt as its value, so that the containers of an attempt that
// failed can be found and removed, even if Testcontainers did not hand them
// out.
const startLabel = "io.eventsourcingdb.container-start"

// pollInterval is how long Start waits between two pings of the database, and
// pingTimeout how long it waits for the answer to one of them.
const (
	pollInterval = 100 * time.Millisecond
	pingTimeout  = time.Second
)

// logTailLines is how many lines from the end of its log the error of a
// container that exited names.
const logTailLines = 20

// createGracePeriod is how long an attempt waits at most for Docker to finish
// creating a container once the context of Start ended, or once the startup
// timeout ran out, before it stops waiting and removes the container in the
// background.
const createGracePeriod = 5 * time.Second

// errStartupTimedOut is the cause with which the context of an attempt ends
// once the startup timeout ran out, so that Start can tell it from any other
// end of the context.
var errStartupTimedOut = errors.New("startup timed out")

// errExited reports that a container exited before the database in it became
// reachable.
var errExited = errors.New("failed to start the container, it exited before the database became reachable")

// startContainer creates and starts a container. It is a variable only so
// that tests can replace it.
var startContainer = testcontainers.GenericContainer

// removeContainers removes the containers of an attempt. It is a variable
// only so that tests can replace it.
var removeContainers = removeContainersOfAttempt

type Container struct {
	imageName      string
	imageTag       string
	internalPort   int
	apiToken       string
	signingKey     *ed25519.PrivateKey
	startupTimeout time.Duration
	container      testcontainers.Container
}

func NewContainer() *Container {
	return &Container{
		imageName:      "thenativeweb/eventsourcingdb",
		imageTag:       "latest",
		internalPort:   3000,
		apiToken:       "secret",
		signingKey:     nil,
		startupTimeout: defaultStartupTimeout,
	}
}

func (c *Container) WithImageTag(tag string) *Container {
	c.imageTag = tag
	return c
}

func (c *Container) WithAPIToken(token string) *Container {
	c.apiToken = token
	return c
}

func (c *Container) WithSigningKey() *Container {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	c.signingKey = &privateKey
	return c
}

func (c *Container) WithPort(port int) *Container {
	c.internalPort = port
	return c
}

// WithStartupTimeout sets how long Start waits for the database in a container
// to become reachable, from the time Docker begins to create the container
// until the database answers a ping. The default is 30 seconds. Pulling the
// image does not count.
//
// The startup timeout must be greater than zero, or Start returns an error.
func (c *Container) WithStartupTimeout(startupTimeout time.Duration) *Container {
	c.startupTimeout = startupTimeout
	return c
}

// Start starts a container with the database, and waits until the database
// answers a ping with 200 OK, for the startup timeout at most. It asks the
// ping of the v2 API, GET /api/v2/system/ping, and the one of the v1 API,
// GET /api/v1/ping, only if the server answers that with 404 or 405, as a
// server without the v2 API does.
//
// If the container runs, but the database does not become reachable in time,
// e.g. because Docker did not publish its port, Start replaces the container
// with a new one, up to 3 containers in total. If Docker fails, or if the
// container exits, Start returns an error at once, which names the end of the
// log of the container in the latter case. Whenever an attempt fails, Start
// removes its container.
func (c *Container) Start(ctx context.Context) error {
	if c.startupTimeout <= 0 {
		return errors.New("startup timeout must be greater than zero")
	}

	for range maxStartAttempts {
		container, replace, err := c.startAttempt(ctx)
		if err == nil {
			c.container = container
			return nil
		}
		if !replace {
			return err
		}
	}

	return fmt.Errorf(
		"failed to start the container, the database did not become reachable within %s in any of %d containers",
		formatSeconds(c.startupTimeout), maxStartAttempts,
	)
}

// startAttempt starts a container, and waits for the startup timeout at most
// until the database in it is reachable. If that fails, for whatever reason,
// it removes the container, and tells whether to replace it with a new one,
// which is the case only if the startup timeout ran out.
func (c *Container) startAttempt(ctx context.Context) (testcontainers.Container, bool, error) {
	attemptID := rand.Text()

	// Each attempt needs a request of its own, as starting a container reads
	// the files of its request to the end.
	request, err := c.newRequest(attemptID)
	if err != nil {
		return nil, false, err
	}

	attemptCtx := newAttemptContext(ctx, c.startupTimeout)
	defer attemptCtx.release()

	request.LifecycleHooks = append(request.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
		PreCreates: []testcontainers.ContainerRequestHook{
			func(context.Context, testcontainers.ContainerRequest) error {
				return attemptCtx.creating()
			},
		},
		PostCreates: []testcontainers.ContainerHook{
			func(context.Context, testcontainers.Container) error {
				attemptCtx.created()
				return nil
			},
		},
	})

	start := startContainer
	started := make(chan startResult, 1)
	go func() {
		container, err := start(attemptCtx, request)
		attemptCtx.created()
		started <- startResult{container: container, err: err}
	}()

	var result startResult
	select {
	case result = <-started:
	case <-attemptCtx.abandoned:
		// Docker did not finish creating the container within the grace
		// period, so the attempt does not wait for it any longer, and removes
		// its containers in the background.
		go removeAbandonedAttempt(started, attemptID)

		replace, err := c.failure(ctx, attemptCtx, nil)
		return nil, replace, err
	}

	container, err := result.container, result.err
	if err == nil {
		err = c.waitUntilReachable(ctx, attemptCtx, container)
		if err == nil {
			return container, false, nil
		}
	}

	replace, err := c.failure(ctx, attemptCtx, err)

	// If removing the container fails, that is named after the error of the
	// attempt, which stays as it is.
	removeErr := removeAttempt(ctx, container, attemptID)
	if removeErr != nil {
		return nil, false, errors.Join(err, fmt.Errorf("failed to remove the container: %w", removeErr))
	}

	return nil, replace, err
}

// startResult is what creating and starting a container returned.
type startResult struct {
	container testcontainers.Container
	err       error
}

// failure returns the error of an attempt that failed with the given error,
// and tells whether to replace its container with a new one, which is the
// case only if the startup timeout ran out.
func (c *Container) failure(ctx context.Context, attemptCtx *attemptContext, err error) (bool, error) {
	switch {
	case errors.Is(err, errExited):
		// The error names the end of the log of the container already.
		return false, err
	case ctx.Err() != nil:
		return false, fmt.Errorf("failed to start the container: %w", ctx.Err())
	case errors.Is(context.Cause(attemptCtx), errStartupTimedOut):
		// This includes a container that Docker is still creating or
		// starting when the time runs out.
		return true, fmt.Errorf("failed to start the container, the database did not become reachable within %s", formatSeconds(c.startupTimeout))
	default:
		return false, fmt.Errorf("failed to start the container: %w", err)
	}
}

// removeAttempt removes the containers of a failed attempt, even if the
// context of Start ended. A container that Testcontainers handed out is
// terminated, so that Testcontainers releases what it holds for it. Beyond
// that, the containers of the attempt are found by their label, as a start
// that fails while Docker starts a container may not hand it out.
func removeAttempt(ctx context.Context, container testcontainers.Container, attemptID string) error {
	return errors.Join(
		testcontainers.TerminateContainer(container),
		removeContainers(context.WithoutCancel(ctx), attemptID),
	)
}

// removeAbandonedAttempt removes the containers of an attempt that stopped
// waiting for Docker to create its container: once the call to create it
// returns, and a final time after the grace period, in case Docker creates
// the container only after the call was cut short. As nobody waits for it, it
// logs what fails.
func removeAbandonedAttempt(started <-chan startResult, attemptID string) {
	var err error

	gracePeriod := time.After(createGracePeriod)
	select {
	case result := <-started:
		err = errors.Join(
			testcontainers.TerminateContainer(result.container),
			removeContainersOfAttempt(context.Background(), attemptID),
		)
		<-gracePeriod
	case <-gracePeriod:
	}

	err = errors.Join(err, removeContainersOfAttempt(context.Background(), attemptID))
	if err != nil {
		log.Printf("failed to remove the container of a start that Docker did not finish in time: %s", err)
	}
}

// attemptContext is the context of an attempt. It ends once the context of
// Start ended, or once the startup timeout ran out, which starts once Docker
// is about to create the container, so that pulling the image does not count.
//
// If it would end while Docker creates the container, it ends only once Docker
// created it, as Docker might otherwise create the container only after the
// attempt looked for it to remove it. It waits for that for the grace period
// at most, and closes abandoned once the grace period ran out.
type attemptContext struct {
	context.Context

	cancel         context.CancelCauseFunc
	startupTimeout time.Duration
	stopParent     func() bool
	abandoned      chan struct{}

	mu          sync.Mutex
	isCreating  bool
	cause       error
	clock       *time.Timer
	gracePeriod *time.Timer
	released    bool
}

func newAttemptContext(parent context.Context, startupTimeout time.Duration) *attemptContext {
	ctx, cancel := context.WithCancelCause(context.WithoutCancel(parent))

	attemptCtx := &attemptContext{
		Context:        ctx,
		cancel:         cancel,
		startupTimeout: startupTimeout,
		abandoned:      make(chan struct{}),
	}
	attemptCtx.stopParent = context.AfterFunc(parent, func() {
		attemptCtx.end(context.Cause(parent))
	})

	return attemptCtx
}

// creating tells that Docker is about to create the container, which starts
// the clock. It returns an error if the context ended already, so that Docker
// does not create the container at all.
func (a *attemptContext) creating() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cause != nil {
		return a.cause
	}

	a.isCreating = true
	a.startClock()
	return nil
}

// created tells that Docker created the container, or failed to. It starts
// the clock if it has not started yet.
func (a *attemptContext) created() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.isCreating = false
	if a.cause != nil {
		a.cancel(a.cause)
	}
	a.startClock()
}

// startClock starts the clock, unless it started already. It must be called
// with the lock held.
func (a *attemptContext) startClock() {
	if a.clock != nil || a.released {
		return
	}

	a.clock = time.AfterFunc(a.startupTimeout, func() {
		a.end(errStartupTimedOut)
	})
}

// end ends the context with the given cause, unless it ended already, at once
// if Docker does not create the container right now, and once Docker created
// it, but within the grace period, otherwise.
func (a *attemptContext) end(cause error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cause != nil || a.released {
		return
	}

	a.cause = cause
	if !a.isCreating {
		a.cancel(cause)
	}

	a.gracePeriod = time.AfterFunc(createGracePeriod, func() {
		a.cancel(cause)
		close(a.abandoned)
	})
}

// release stops the clock and the grace period, and ends the context.
func (a *attemptContext) release() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.released = true
	a.stopParent()
	if a.clock != nil {
		a.clock.Stop()
	}
	if a.gracePeriod != nil {
		a.gracePeriod.Stop()
	}
	a.cancel(nil)
}

// newRequest returns a request for a container with the database, as it is
// configured, which carries the label of the given attempt.
func (c *Container) newRequest(attemptID string) (testcontainers.GenericContainerRequest, error) {
	files := []testcontainers.ContainerFile{}

	cmd := []string{
		"run",
		"--api-token", c.apiToken,
		"--data-directory-temporary",
		"--http-enabled",
		"--https-enabled=false",
		"--http-port", strconv.Itoa(c.internalPort),
	}

	if c.signingKey != nil {
		signingKeyBytes, err := x509.MarshalPKCS8PrivateKey(*c.signingKey)
		if err != nil {
			return testcontainers.GenericContainerRequest{}, err
		}

		block := &pem.Block{Type: "PRIVATE KEY", Bytes: signingKeyBytes}
		pemBytes := pem.EncodeToMemory(block)
		reader := bytes.NewReader(pemBytes)

		targetPath := "/etc/esdb/signing-key.pem"

		files = append(files, testcontainers.ContainerFile{
			Reader:            reader,
			ContainerFilePath: targetPath,
			FileMode:          0777,
		})
		cmd = append(cmd, "--signing-key-file", targetPath)
	}

	request := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("%s:%s", c.imageName, c.imageTag),
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", c.internalPort)},
		Labels:       map[string]string{startLabel: attemptID},
		Files:        files,
		Cmd:          cmd,
	}

	return testcontainers.GenericContainerRequest{
		ContainerRequest: request,
		Started:          true,
	}, nil
}

// waitUntilReachable pings the database in a container until it answers with
// 200 OK, until the context of the attempt ends, until the container exits, or
// until Docker fails. It reads the port that Docker published for the
// database anew for each ping, as Docker may publish it only a while after it
// started the container. The log of a container that exited is read with the
// context of Start, which the startup timeout does not end.
func (c *Container) waitUntilReachable(ctx, attemptCtx context.Context, container testcontainers.Container) error {
	port := fmt.Sprintf("%d/tcp", c.internalPort)
	httpClient := &http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
		Timeout:   pingTimeout,
	}

	for {
		state, err := container.State(attemptCtx)
		if err != nil {
			return err
		}
		if state.Status == dockercontainer.StateExited || state.Status == dockercontainer.StateDead {
			return exited(ctx, container)
		}
		if state.Running {
			reachable, err := answersPing(attemptCtx, httpClient, container, port)
			if err != nil {
				return err
			}
			if reachable {
				return nil
			}
		}

		select {
		case <-attemptCtx.Done():
			return attemptCtx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// answersPing tells whether the database in a container answers a ping with
// 200 OK on the host port that Docker published for its port, if it did. It
// asks the ping of the v2 API, and the one of the v1 API only if the database
// answers that with 404 or 405, as a server without the v2 API does, so that
// it works against servers with and without either API. Any other outcome of
// the ping of the v2 API, e.g. a refused connection while the database starts,
// does not lead to the ping of the v1 API, but to the next ping, which asks
// the v2 API again. It returns an error only if Docker fails, or if no ping
// can be built from the host and the port that Docker tells.
//
// It does not ping with the client of this module, so that a defect in how the
// client chooses the API fails the tests that pin it, rather than every test
// that starts a container.
func answersPing(ctx context.Context, httpClient *http.Client, container testcontainers.Container, port string) (bool, error) {
	host, err := container.Host(ctx)
	if err != nil {
		return false, err
	}
	mappedPort, err := container.MappedPort(ctx, port)
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	baseURL := "http://" + net.JoinHostPort(host, strconv.Itoa(int(mappedPort.Num())))

	statusCode, err := pingStatus(ctx, httpClient, baseURL+"/api/v2/system/ping")
	if err != nil {
		return false, err
	}
	if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed {
		statusCode, err = pingStatus(ctx, httpClient, baseURL+"/api/v1/ping")
		if err != nil {
			return false, err
		}
	}

	return statusCode == http.StatusOK, nil
}

// pingStatus asks the database for the ping at the given URL, and returns the
// status code of its answer, or 0 if it does not answer, e.g. as it is still
// starting, or as it did not answer within the bound per request. It returns
// an error only if the request can not be built.
func pingStatus(ctx context.Context, httpClient *http.Client, pingURL string) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return 0, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return 0, nil
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)

	return response.StatusCode, nil
}

// exited returns the error for a container that exited, which names the last
// lines of its log.
func exited(ctx context.Context, container testcontainers.Container) error {
	logs, err := container.Logs(ctx)
	if err != nil {
		return fmt.Errorf("%w, and failed to read its log: %w", errExited, err)
	}
	defer logs.Close()

	output, err := io.ReadAll(logs)
	if err != nil {
		return fmt.Errorf("%w, and failed to read its log: %w", errExited, err)
	}

	lines := strings.Split(strings.TrimRight(string(output), "\r\n"), "\n")
	if len(lines) > logTailLines {
		lines = lines[len(lines)-logTailLines:]
	}

	return fmt.Errorf("%w: %s", errExited, strings.Join(lines, "\n"))
}

// removeContainersOfAttempt removes every container that carries the label of
// the given attempt, whether it runs or not.
func removeContainersOfAttempt(ctx context.Context, attemptID string) error {
	dockerClient, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	containers, err := dockerClient.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("label", startLabel+"="+attemptID),
	})
	if err != nil {
		return err
	}

	for _, container := range containers.Items {
		_, err := dockerClient.ContainerRemove(ctx, container.ID, client.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
		if err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// formatSeconds writes a duration in seconds, as an integer if it is whole,
// and as short as possible otherwise, e.g. "30s" or "1.5s".
func formatSeconds(duration time.Duration) string {
	return strconv.FormatFloat(duration.Seconds(), 'f', -1, 64) + "s"
}

func (c *Container) GetHost(ctx context.Context) (string, error) {
	if c.container == nil {
		return "", errors.New("container must be running")
	}
	return c.container.Host(ctx)
}

func (c *Container) GetMappedPort(ctx context.Context) (int, error) {
	if c.container == nil {
		return 0, errors.New("container must be running")
	}

	port, err := c.container.MappedPort(ctx, fmt.Sprintf("%d/tcp", c.internalPort))
	if err != nil {
		return 0, err
	}
	return int(port.Num()), nil
}

func (c *Container) GetBaseURL(ctx context.Context) (*url.URL, error) {
	host, err := c.GetHost(ctx)
	if err != nil {
		return nil, err
	}

	port, err := c.GetMappedPort(ctx)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return nil, err
	}

	return baseURL, nil
}

func (c *Container) GetAPIToken() string {
	return c.apiToken
}

func (c *Container) GetSigningKey() (*ed25519.PrivateKey, error) {
	if c.signingKey == nil {
		return nil, errors.New("signing key not set")
	}

	return c.signingKey, nil
}

func (c *Container) GetVerificationKey() (*ed25519.PublicKey, error) {
	if c.signingKey == nil {
		return nil, errors.New("signing key not set")
	}

	verificationKey, ok := c.signingKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("failed to get verification key from signing key")
	}

	return &verificationKey, nil
}

func (c *Container) IsRunning() bool {
	return c.container != nil
}

func (c *Container) Stop(ctx context.Context) error {
	if c.container == nil {
		return nil
	}

	err := c.container.Terminate(ctx)
	if err != nil {
		return err
	}

	c.container = nil
	return nil
}

func (c *Container) GetClient(ctx context.Context) (*eventsourcingdb.Client, error) {
	baseURL, err := c.GetBaseURL(ctx)
	if err != nil {
		return nil, err
	}

	client, err := eventsourcingdb.NewClient(baseURL, c.apiToken)
	if err != nil {
		return nil, err
	}

	return client, nil
}
