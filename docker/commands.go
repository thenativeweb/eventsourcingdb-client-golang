package docker

import (
	"os/exec"
	"strconv"
	"strings"
)

func BuildImage(directory string, image Image) error {
	cliCommand := exec.Command("docker", "build", "-t", image.GetFullName(), directory)

	err := cliCommand.Run()
	if err != nil {
		return err
	}

	return nil
}

func RunContainer(image Image, command string, detached, exposePorts bool, args ...string) (Container, error) {
	commandArgs := []string{"run", "--rm"}
	if detached {
		commandArgs = append(commandArgs, "-d")
	}
	if exposePorts {
		commandArgs = append(commandArgs, "-P")
	}
	commandArgs = append(commandArgs, image.GetFullName())
	commandArgs = append(commandArgs, command)
	commandArgs = append(commandArgs, args...)

	cliCommand := exec.Command("docker", commandArgs...)
	stdout, err := cliCommand.Output()
	if err != nil {
		return Container{}, err
	}

	containerID := strings.TrimSpace(string(stdout))
	container := Container{containerID}

	return container, nil
}

func KillContainer(container Container) error {
	cliCommand := exec.Command("docker", "kill", container.ID)
	err := cliCommand.Run()
	if err != nil {
		return err
	}

	return nil
}

func GetExposedPort(container Container, internalPort int) (int, error) {
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
