package docker

import (
	"fmt"
	"os/exec"
	"strings"
)

type Image struct {
	Name string
	Tag  string
}

func (image Image) GetFullName() string {
	return image.Name + ":" + image.Tag
}

func (image Image) Build(directory string) error {
	cliCommand := exec.Command("docker", "build", "-t", image.GetFullName(), directory)

	_, err := cliCommand.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("failed to build image: %s", exitError.Stderr)
		}

		return err
	}

	return nil
}

func (image Image) Run(command []string, detached, exposePorts bool) (Container, error) {
	commandArgs := []string{"run", "--rm"}
	if detached {
		commandArgs = append(commandArgs, "-d")
	}
	if exposePorts {
		commandArgs = append(commandArgs, "-P")
	}
	commandArgs = append(commandArgs, image.GetFullName())
	commandArgs = append(commandArgs, command...)

	cliCommand := exec.Command("docker", commandArgs...)

	stdout, err := cliCommand.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return Container{}, fmt.Errorf("failed to run image: %s", exitError.Stderr)
		}
		return Container{}, fmt.Errorf("failed to run image: %s", stderr)
	}

	containerID := strings.TrimSpace(string(stdout))
	container := Container{containerID}

	return container, nil
}
