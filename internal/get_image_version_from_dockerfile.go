package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var versionRegex = regexp.MustCompile(`(?m)^FROM\sthenativeweb\/eventsourcingdb:(.+)$`)

func GetImageVersionFromDockerfile() (string, error) {
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
