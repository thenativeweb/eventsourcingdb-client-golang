package docker

import (
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

	err := cliCommand.Run()
	if err != nil {
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
		return Container{}, err
	}

	containerID := strings.TrimSpace(string(stdout))
	container := Container{containerID}

	return container, nil
}
