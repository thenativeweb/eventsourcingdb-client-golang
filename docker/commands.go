package docker

import (
	"os/exec"
	"strconv"
	"strings"
)

func getImageWithTag(image, tag string) string {
	return image + ":" + tag
}

func PullImage(image, tag string) error {
	imageWithTag := getImageWithTag(image, tag)
	cliCommand := exec.Command("docker", "pull", imageWithTag)

	err := cliCommand.Run()
	if err != nil {
		return err
	}

	return nil
}

func RunContainer(image, tag, command string, detached, exposePorts bool, args ...string) (string, error) {
	imageWithTag := getImageWithTag(image, tag)

	commandArgs := []string{"run", "--rm"}
	if detached {
		commandArgs = append(commandArgs, "-d")
	}
	if exposePorts {
		commandArgs = append(commandArgs, "-P")
	}
	commandArgs = append(commandArgs, imageWithTag)
	commandArgs = append(commandArgs, command)
	commandArgs = append(commandArgs, args...)

	cliCommand := exec.Command("docker", commandArgs...)
	stdout, err := cliCommand.Output()
	if err != nil {
		return "", err
	}

	containerID := strings.TrimSpace(string(stdout))

	return containerID, nil
}

func KillContainer(containerID string) error {
	cliCommand := exec.Command("docker", "kill", containerID)
	err := cliCommand.Run()
	if err != nil {
		return err
	}

	return nil
}

func GetExposedPort(containerID string, internalPort int) (int, error) {
	cliCommand := exec.Command("docker", "inspect", "--format='{{(index (index .NetworkSettings.Ports \""+strconv.Itoa(internalPort)+"/tcp\") 0).HostPort}}'", containerID)
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
