package test

import (
	"context"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
	"strconv"
)

type ContainerizedTestingDatabase struct {
	TestingDatabase

	image   docker.Image
	options eventsourcingdb.ClientOptions
	command []string

	container docker.Container

	isFirstRun bool
}

func NewContainerizedTestingDatabase(image docker.Image, command []string, clientOptions eventsourcingdb.ClientOptions) (ContainerizedTestingDatabase, error) {
	database := ContainerizedTestingDatabase{
		TestingDatabase: TestingDatabase{},
		image:           image,
		options:         clientOptions,
		command:         command,
		container:       docker.Container{},
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

func (database *ContainerizedTestingDatabase) GetClient() eventsourcingdb.Client {
	if database.isFirstRun {
		database.isFirstRun = false

		return database.client
	}

	database.restart()

	return database.client
}

type startResult struct {
	client    eventsourcingdb.Client
	container docker.Container
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
	client := eventsourcingdb.NewClientWithOptions(baseURL, database.options)

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
