package eventsourcingdb_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// The tests run against the image that EVENTSOURCINGDB_IMAGE_TAG names, so
// that the pipeline can run them against every tag that matters, and against
// the image of the Dockerfile without it, so that a local run needs nothing
// to be set.
func TestGetImageTag(t *testing.T) {
	t.Run("returns the tag that the environment variable sets", func(t *testing.T) {
		t.Setenv(imageTagEnvironmentVariable, "1.2.0")

		imageTag, err := getImageTag()

		require.NoError(t, err)
		assert.Equal(t, "1.2.0", imageTag)
	})

	t.Run("returns the tag of the Dockerfile if the environment variable is not set", func(t *testing.T) {
		t.Setenv(imageTagEnvironmentVariable, "")
		os.Unsetenv(imageTagEnvironmentVariable)

		imageTag, err := getImageTag()

		require.NoError(t, err)
		assert.Equal(t, imageTagOfDockerfile(t), imageTag)
	})

	t.Run("returns the tag of the Dockerfile if the environment variable is empty", func(t *testing.T) {
		t.Setenv(imageTagEnvironmentVariable, "")

		imageTag, err := getImageTag()

		require.NoError(t, err)
		assert.Equal(t, imageTagOfDockerfile(t), imageTag)
	})
}

func imageTagOfDockerfile(t *testing.T) string {
	t.Helper()

	dockerfile, err := os.ReadFile(filepath.Join("..", "docker", "Dockerfile"))
	require.NoError(t, err)

	matches := regexp.MustCompile(`(?m)^FROM thenativeweb/eventsourcingdb:(.+)$`).FindSubmatch(dockerfile)
	require.Len(t, matches, 2)

	return string(matches[1])
}
