package docker

import (
	"os/exec"
	"strconv"
	"strings"
)

type Container struct {
	ID string
}

func (container Container) Kill() error {
	cliCommand := exec.Command("docker", "kill", container.ID)
	err := cliCommand.Run()
	if err != nil {
		return err
	}

	return nil
}

func (container Container) GetExposedPort(internalPort int) (int, error) {
	cliCommand := exec.Command("docker", "inspect", "--format='{{(index (index .NetworkSettings.Ports \""+strconv.Itoa(internalPort)+"/tcp\") 0).HostPort}}'", container.ID)
	stdout, err := cliCommand.Output()
	if err != nil {
		return 0, err
	}

	exposedPortAsString := strings.Trim(string(stdout), " '\n\r")
	exposedPort, err := strconv.Atoi(exposedPortAsString)
	if err != nil {
		return 0, err
	}

	return exposedPort, nil
}
