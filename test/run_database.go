package test

import (
	"context"
	"strconv"

	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
)

func runDatabase(runContainerFn func() (container docker.Container, err error), clientOptions eventsourcingdb.ClientOptions) (docker.Container, string, eventsourcingdb.Client, error) {
	container, err := runContainerFn()
	if err != nil {
		return docker.Container{}, "", eventsourcingdb.Client{}, err
	}

	port, err := container.GetExposedPort(3000)
	if err != nil {
		return docker.Container{}, "", eventsourcingdb.Client{}, err
	}

	baseURL := "http://localhost:" + strconv.Itoa(port)
	client := eventsourcingdb.NewClientWithOptions(baseURL, clientOptions)

	err = retry.WithBackoff(context.Background(), 10, func() error {
		return client.Ping()
	})
	if err != nil {
		return docker.Container{}, "", eventsourcingdb.Client{}, err
	}

	return container, baseURL, client, nil
}
