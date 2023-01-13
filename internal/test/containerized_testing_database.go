package test

import (
	"context"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
	docker2 "github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/docker"
	client2 "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"strconv"
)

type ContainerizedTestingDatabase struct {
	TestingDatabase

	image   docker2.Image
	options client2.ClientOptions
	command []string

	container docker2.Container

	isFirstRun bool
}

func NewContainerizedTestingDatabase(image docker2.Image, command []string, clientOptions client2.ClientOptions) (ContainerizedTestingDatabase, error) {
	database := ContainerizedTestingDatabase{
		TestingDatabase: TestingDatabase{},
		image:           image,
		options:         clientOptions,
		command:         command,
		container:       docker2.Container{},
		isFirstRun:      true,
	}

	startResult, err := database.start()
	if err != nil {
		return database, err
	}

	database.container = startResult.container
	database.client = startResult.client

	err = retry.WithBackoff(context.Background(), 10, func() error {
		return database.client.Ping()
	})
	if err != nil {
		return ContainerizedTestingDatabase{}, err
	}

	return database, nil
}

func (database *ContainerizedTestingDatabase) GetClient() client2.Client {
	if database.isFirstRun {
		database.isFirstRun = false

		return database.client
	}

	database.restart()

	return database.client
}

type startResult struct {
	client    client2.Client
	container docker2.Container
}

func (database *ContainerizedTestingDatabase) start() (startResult, error) {
	container, err := database.image.Run(database.command, true, true)
	if err != nil {
		return startResult{}, err
	}

	port, err := container.GetExposedPort(3000)
	if err != nil {
		return startResult{}, err
	}

	baseURL := "http://localhost:" + strconv.Itoa(port)
	client := client2.NewClientWithOptions(baseURL, database.options)

	err = retry.WithBackoff(context.Background(), 10, func() error {
		return client.Ping()
	})
	if err != nil {
		return startResult{}, err
	}

	return startResult{
		container: container,
		client:    client,
	}, nil
}

func (database *ContainerizedTestingDatabase) restart() {
	if err := database.container.Kill(); err != nil {
		panic("could not kill database container")
	}

	startResult, err := database.start()
	if err != nil {
		panic("could not restart database container")
	}

	database.client = startResult.client
	database.container = startResult.container
}

func (database *ContainerizedTestingDatabase) Stop() error {
	if err := database.container.Kill(); err != nil {
		return err
	}

	return nil
}
