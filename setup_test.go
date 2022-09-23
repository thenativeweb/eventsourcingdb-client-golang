package eventsourcingdb_test

import (
	"log"
	"os"
	"strconv"
	"testing"

	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
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

	exitCode = m.Run()
}
