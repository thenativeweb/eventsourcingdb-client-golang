package eventsourcingdb_test

import (
	"context"
	"github.com/google/uuid"
	"log"
	"os"
	"strconv"
	"testing"

	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
)

var accessToken = uuid.New().String()

var baseURLWithAuthorization = ""
var baseURLWithoutAuthorization = ""

func runAndWaitForContainer(runContainerFn func() (containerID string, err error)) (containerID string, baseURL string, err error) {
	containerID, err = runContainerFn()
	if err != nil {
		return "", "", err
	}

	port, err := docker.GetExposedPort(containerID, 3000)
	if err != nil {
		return "", "", err
	}

	baseURL = "http://localhost:" + strconv.Itoa(port)
	client := eventsourcingdb.NewClient(baseURL)

	err = retry.WithBackoff(func() error {
		return client.Ping()
	}, 10, context.Background())
	if err != nil {
		return "", "", err
	}

	return containerID, baseURL, nil
}

func TestMain(m *testing.M) {
	exitCode := 0
	runningContainerIDs := []string{}

	defer func() {
		for _, runningContainerID := range runningContainerIDs {
			err := docker.KillContainer(runningContainerID)
			if err != nil {
				log.Println(err)
			}
		}

		os.Exit(exitCode)
	}()

	err := docker.PullImage("ghcr.io/thenativeweb/eventsourcingdb", "latest")
	if err != nil {
		log.Println(err)
		return
	}

	var containerIDWithAuthorization string
	containerIDWithAuthorization, baseURLWithAuthorization, err = runAndWaitForContainer(func() (containerID string, err error) {
		return docker.RunContainer("ghcr.io/thenativeweb/eventsourcingdb", "latest", "server", true, true, "--dev", "--access-token", accessToken)
	})
	if err != nil {
		log.Println(err)
		return
	}
	runningContainerIDs = append(runningContainerIDs, containerIDWithAuthorization)

	var containerIDWithoutAuthorization string
	containerIDWithoutAuthorization, baseURLWithoutAuthorization, err = runAndWaitForContainer(func() (containerID string, err error) {
		return docker.RunContainer("ghcr.io/thenativeweb/eventsourcingdb", "latest", "server", true, true, "--dev")
	})
	if err != nil {
		log.Println(err)
		return
	}
	runningContainerIDs = append(runningContainerIDs, containerIDWithoutAuthorization)

	exitCode = m.Run()
}
