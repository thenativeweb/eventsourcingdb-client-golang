package eventsourcingdb_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
)

var baseUrl = ""

func TestMain(m *testing.M) {
	exitCode := 0

	cmd := exec.Command("docker", "pull", "ghcr.io/thenativeweb/eventsourcingdb:latest")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	cmd = exec.Command("docker", "run", "--rm", "-d", "-P", "ghcr.io/thenativeweb/eventsourcingdb:latest", "server", "--dev")
	stdout, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	containerID := strings.TrimSpace(string(stdout))

	defer func() {
		cmd = exec.Command("docker", "kill", containerID)
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		os.Exit(exitCode)
	}()

	cmd = exec.Command("docker", "inspect", "--format='{{(index (index .NetworkSettings.Ports \"3000/tcp\") 0).HostPort}}'", containerID)
	stdout, err = cmd.Output()
	if err != nil {
		fmt.Println(stdout)
		log.Fatal(err)
	}

	port := strings.Trim(string(stdout), " '\n\r")
	baseUrl = "http://localhost:" + port

	exitCode = m.Run()
}
