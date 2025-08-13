package eventsourcingdb

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
	files := []testcontainers.ContainerFile{}

	cmd := []string{
		"run",
		"--api-token", c.apiToken,
		"--data-directory-temporary",
		"--http-enabled",
		"--https-enabled=false",
	}

	if c.signingKey != nil {
		signingKeyBytes, err := x509.MarshalPKCS8PrivateKey(*c.signingKey)
		if err != nil {
			return err
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

func (c *Container) GetClient(ctx context.Context) (*Client, error) {
	baseURL, err := c.GetBaseURL(ctx)
	if err != nil {
		return nil, err
	}

	client, err := NewClient(baseURL, c.apiToken)
	if err != nil {
		return nil, err
	}

	return client, nil
}
