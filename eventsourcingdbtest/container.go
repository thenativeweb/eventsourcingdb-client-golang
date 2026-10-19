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
	"time"

	"github.com/containerd/errdefs"
	dockercontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
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
// to become reachable, from the time Docker created the container until the
// database answers a ping. The default is 30 seconds. Pulling the image does
// not count.
//
// The startup timeout must be greater than zero, or Start returns an error.
func (c *Container) WithStartupTimeout(startupTimeout time.Duration) *Container {
	c.startupTimeout = startupTimeout
	return c
}

// Start starts a container with the database, and waits until the database
// answers a ping with 200 OK, for the startup timeout at most.
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

	attemptCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// The clock starts once Docker created the container, so that neither
	// pulling the image nor starting the reaper of Testcontainers counts. It
	// starts at the latest once the container was started.
	var clock *time.Timer
	startClock := func() {
		if clock == nil {
			clock = time.AfterFunc(c.startupTimeout, func() {
				cancel(errStartupTimedOut)
			})
		}
	}
	defer func() {
		if clock != nil {
			clock.Stop()
		}
	}()

	request.LifecycleHooks = append(request.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
		PostCreates: []testcontainers.ContainerHook{
			func(context.Context, testcontainers.Container) error {
				startClock()
				return nil
			},
		},
	})

	container, err := startContainer(attemptCtx, request)
	if err == nil {
		startClock()

		err = c.waitUntilReachable(ctx, attemptCtx, container)
		if err == nil {
			return container, false, nil
		}
	}

	replace := false
	switch {
	case errors.Is(err, errExited):
		// The error names the end of the log of the container already.
	case ctx.Err() != nil:
		err = fmt.Errorf("failed to start the container: %w", ctx.Err())
	case errors.Is(context.Cause(attemptCtx), errStartupTimedOut):
		// This includes a container that Docker is still creating or
		// starting when the time runs out.
		replace = true
		err = fmt.Errorf("failed to start the container, the database did not become reachable within %s", formatSeconds(c.startupTimeout))
	default:
		err = fmt.Errorf("failed to start the container: %w", err)
	}

	// The containers of the attempt are found by their label, since a start
	// that fails while Docker creates or starts a container may not hand it
	// out. They are removed even if the context of Start ended.
	removeErr := removeContainers(context.WithoutCancel(ctx), attemptID)
	if removeErr != nil {
		return nil, false, fmt.Errorf("%w, and failed to remove the container: %w", err, removeErr)
	}

	return nil, replace, err
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
// 200 OK, until the context of the attempt ends, or until the container
// exits. It reads the port that Docker published for the database anew for
// each ping, as Docker may publish it only a while after it started the
// container. The log of a container that exited is read with the context of
// Start, which the startup timeout does not end.
func (c *Container) waitUntilReachable(ctx, attemptCtx context.Context, container testcontainers.Container) error {
	port := fmt.Sprintf("%d/tcp", c.internalPort)
	httpClient := &http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
		Timeout:   pingTimeout,
	}

	for {
		state, err := container.State(attemptCtx)
		if err == nil && (state.Status == dockercontainer.StateExited || state.Status == dockercontainer.StateDead) {
			return exited(ctx, container)
		}
		if err == nil && state.Running && answersPing(attemptCtx, httpClient, container, port) {
			return nil
		}

		select {
		case <-attemptCtx.Done():
			return attemptCtx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// answersPing tells whether the database in a container answers a ping with
// 200 OK on the host port that Docker published for its port, if it did.
func answersPing(ctx context.Context, httpClient *http.Client, container testcontainers.Container, port string) bool {
	host, err := container.Host(ctx)
	if err != nil {
		return false
	}
	mappedPort, err := container.MappedPort(ctx, port)
	if err != nil {
		return false
	}

	pingURL := fmt.Sprintf("http://%s/api/v1/ping", net.JoinHostPort(host, strconv.Itoa(int(mappedPort.Num()))))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return false
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)

	return response.StatusCode == http.StatusOK
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
