package eventsourcingdbtest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// imageTagEnvironmentVariable names the environment variable that sets the
// tag of the EventSourcingDB image the tests run against, e.g. latest or
// preview.
const imageTagEnvironmentVariable = "EVENTSOURCINGDB_IMAGE_TAG"

var versionRegex = regexp.MustCompile(`(?m)^FROM\sthenativeweb\/eventsourcingdb:(.+)$`)

// getImageTag returns the tag of the EventSourcingDB image the tests run
// against: the value of EVENTSOURCINGDB_IMAGE_TAG if it is set and not empty,
// and the tag in docker/Dockerfile otherwise.
func getImageTag() (string, error) {
	imageTag := os.Getenv(imageTagEnvironmentVariable)
	if imageTag != "" {
		return imageTag, nil
	}

	return getImageTagFromDockerfile()
}

func getImageTagFromDockerfile() (string, error) {
	dockerfile := filepath.Join("..", "docker", "Dockerfile")
	dataBytes, err := os.ReadFile(dockerfile)
	if err != nil {
		return "", err
	}

	data := string(dataBytes)

	matches := versionRegex.FindStringSubmatch(data)
	if matches == nil {
		return "", fmt.Errorf("failed to find image version in Dockerfile")
	}
	if len(matches) != 2 {
		return "", fmt.Errorf("unexpected format in Dockerfile")
	}

	return matches[1], nil
}
