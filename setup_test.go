package eventsourcingdb_test

import (
	"context"
	"log"
	"os"
	"strconv"
	"testing"

	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
)

var baseUrl = ""

func TestMain(m *testing.M) {
	exitCode := 0

	err := docker.PullImage("ghcr.io/thenativeweb/eventsourcingdb", "latest")
	if err != nil {
		log.Fatal(err)
	}

	containerID, err := docker.RunContainer("ghcr.io/thenativeweb/eventsourcingdb", "latest", "server", true, true, "--dev")
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err := docker.KillContainer(containerID)
		if err != nil {
			log.Fatal(err)
		}

		os.Exit(exitCode)
	}()

	port, err := docker.GetExposedPort(containerID, 3000)
	if err != nil {
		log.Fatal(err)
	}

	baseUrl = "http://localhost:" + strconv.Itoa(port)
	client := eventsourcingdb.NewClient(baseUrl)

	err = retry.WithBackoff(func() error {
		return client.Ping()
	}, 10, context.Background())
	if err != nil {
		log.Fatal(err)
	}

	exitCode = m.Run()
}
