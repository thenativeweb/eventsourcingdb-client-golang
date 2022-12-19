package docker

import (
	"bytes"
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

	stderrBuf := bytes.NewBuffer([]byte{})
	cliCommand.Stderr = stderrBuf

	err := cliCommand.Run()
	if err != nil {
		stderr := string(stderrBuf.Bytes())
		return fmt.Errorf("failed to build image: %s", stderr)
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

	stderrBuf := bytes.NewBuffer([]byte{})
	cliCommand.Stderr = stderrBuf

	stdout, err := cliCommand.Output()
	if err != nil {
		stderr := string(stderrBuf.Bytes())
		return Container{}, fmt.Errorf("failed to run image: %s", stderr)
	}

	containerID := strings.TrimSpace(string(stdout))
	container := Container{containerID}

	return container, nil
}
