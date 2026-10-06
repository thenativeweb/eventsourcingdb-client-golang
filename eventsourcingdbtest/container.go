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
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
)

// maxStartAttempts is how many containers Start starts at most, as it
// replaces each container whose port Docker did not publish.
const maxStartAttempts = 3

// startContainer creates and starts a container. It is a variable only so
// that tests can replace it.
var startContainer = testcontainers.GenericContainer

type Container struct {
	imageName    string
	imageTag     string
	internalPort int
	apiToken     string
	signingKey   *ed25519.PrivateKey
	container    testcontainers.Container
}

func NewContainer() *Container {
	return &Container{
		imageName:    "thenativeweb/eventsourcingdb",
		imageTag:     "latest",
		internalPort: 3000,
		apiToken:     "secret",
		signingKey:   nil,
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

func (c *Container) Start(ctx context.Context) error {
	port := fmt.Sprintf("%d/tcp", c.internalPort)

	for attempt := 1; ; attempt++ {
		// Each attempt needs a request of its own, as starting a container
		// reads the files of its request to the end.
		request, err := c.newRequest()
		if err != nil {
			return err
		}

		container, err := startContainer(ctx, request)
		if err == nil {
			c.container = container
			return nil
		}

		// Docker Desktop sometimes fails to publish the port it chose for a
		// container on the host, because something on the host took that port
		// in the meantime. The container keeps running, but the database in it
		// can never be reached, so it is replaced by a new one. Every other
		// failure is returned at once. Either way, the container that failed
		// to start is terminated, so that it is not left behind.
		unpublished := isRunningWithoutPublishedPort(ctx, container, port)

		terminateErr := testcontainers.TerminateContainer(container)
		if terminateErr != nil {
			return fmt.Errorf("%w, and failed to terminate container: %w", err, terminateErr)
		}

		if !unpublished {
			return err
		}
		if attempt == maxStartAttempts {
			return fmt.Errorf("failed to start container, Docker did not publish port %s in %d attempts: %w", port, maxStartAttempts, err)
		}
		if ctx.Err() != nil {
			return fmt.Errorf("failed to start container, Docker did not publish port %s, and the context ended before another attempt: %w: %w", port, ctx.Err(), err)
		}
	}
}

// newRequest returns a request for a container with the database, as it is
// configured.
func (c *Container) newRequest() (testcontainers.GenericContainerRequest, error) {
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
		Files:        files,
		Cmd:          cmd,
		WaitingFor: wait.
			ForHTTP("/api/v1/ping").
			WithPort(fmt.Sprintf("%d/tcp", c.internalPort)).
			WithStartupTimeout(10 * time.Second),
	}

	return testcontainers.GenericContainerRequest{
		ContainerRequest: request,
		Started:          true,
	}, nil
}

// isRunningWithoutPublishedPort reports whether a container runs, but Docker
// did not publish the given port of it on the host.
func isRunningWithoutPublishedPort(ctx context.Context, container testcontainers.Container, port string) bool {
	if isNil(container) {
		return false
	}

	inspect, err := container.Inspect(ctx)
	if err != nil || !inspect.State.Running {
		return false
	}

	for containerPort, bindings := range inspect.NetworkSettings.Ports {
		if containerPort.String() == port && len(bindings) > 0 {
			return false
		}
	}

	return true
}

// isNil reports whether a container is nil, also if it is a nil pointer or
// another nil value of a concrete type, which is not nil as an interface. It
// checks this as testcontainers.TerminateContainer does.
func isNil(container testcontainers.Container) bool {
	if container == nil {
		return true
	}

	value := reflect.ValueOf(container)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
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
