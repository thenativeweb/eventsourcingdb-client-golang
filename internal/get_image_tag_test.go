package internal_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
)

// The tests run against the image that EVENTSOURCINGDB_IMAGE_TAG names, so
// that the pipeline can run them against every tag that matters, and against
// the image of the Dockerfile without it, so that a local run needs nothing
// to be set.
func TestGetImageTag(t *testing.T) {
	t.Run("returns the tag that the environment variable sets", func(t *testing.T) {
		t.Setenv(internal.ImageTagEnvironmentVariable, "1.2.0")

		imageTag, err := internal.GetImageTag()

		require.NoError(t, err)
		assert.Equal(t, "1.2.0", imageTag)
	})

	t.Run("returns the tag of the Dockerfile if the environment variable is not set", func(t *testing.T) {
		t.Setenv(internal.ImageTagEnvironmentVariable, "")
		os.Unsetenv(internal.ImageTagEnvironmentVariable)

		imageTag, err := internal.GetImageTag()

		require.NoError(t, err)
		assert.Equal(t, imageTagOfDockerfile(t), imageTag)
	})

	t.Run("returns the tag of the Dockerfile if the environment variable is empty", func(t *testing.T) {
		t.Setenv(internal.ImageTagEnvironmentVariable, "")

		imageTag, err := internal.GetImageTag()

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
