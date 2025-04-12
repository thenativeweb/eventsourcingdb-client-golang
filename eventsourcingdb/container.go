package eventsourcingdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type Container struct {
	imageName    string
	imageTag     string
	internalPort int
	apiToken     string
	container    testcontainers.Container
}

func NewContainer() *Container {
	return &Container{
		imageName:    "thenativeweb/eventsourcingdb",
		imageTag:     "latest",
		internalPort: 3000,
		apiToken:     "secret",
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

func (c *Container) WithPort(port int) *Container {
	c.internalPort = port
	return c
}

func (c *Container) Start(ctx context.Context) error {
	request := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("%s:%s", c.imageName, c.imageTag),
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", c.internalPort)},
		Cmd: []string{
			"run",
			"--api-token", c.apiToken,
			"--data-directory-temporary",
			"--http-enabled",
			"--https-enabled=false",
		},
		WaitingFor: wait.
			ForHTTP("/api/v1/ping").
			WithPort(nat.Port(fmt.Sprintf("%d/tcp", c.internalPort))).
			WithStartupTimeout(10 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: request,
		Started:          true,
	})
	if err != nil {
		return err
	}

	c.container = container
	return nil
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

	natPort := nat.Port(fmt.Sprintf("%d/tcp", c.internalPort))
	port, err := c.container.MappedPort(ctx, natPort)
	if err != nil {
		return 0, err
	}
	return port.Int(), nil
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

func (c *Container) GetClient(ctx context.Context) (*Client, error) {
	baseUrl, err := c.GetBaseURL(ctx)
	if err != nil {
		return nil, err
	}

	client, err := NewClient(baseUrl, c.apiToken)
	if err != nil {
		return nil, err
	}

	return client, nil
}
