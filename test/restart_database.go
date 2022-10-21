package test

import (
	"context"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
	"strconv"
)

func (database *ContainerizedTestingDatabase) restart() (ContainerizedTestingDatabase, error) {
	if err := database.Container.Restart(); err != nil {
		return ContainerizedTestingDatabase{}, err
	}

	port, err := database.Container.GetExposedPort(3000)
	if err != nil {
		return ContainerizedTestingDatabase{}, err
	}

	baseURL := "http://localhost:" + strconv.Itoa(port)
	client := eventsourcingdb.NewClientWithOptions(baseURL, database.clientOptions)

	err = retry.WithBackoff(func() error {
		return client.Ping()
	}, 10, context.Background())
	if err != nil {
		return ContainerizedTestingDatabase{}, err
	}

	return NewContainerizedTestingDatabase(database.clientOptions, client, baseURL, database.Container), nil
}
