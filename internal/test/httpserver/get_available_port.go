package httpserver

import (
	"fmt"
	"net"
)

func isPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))

	if err != nil {
		return false
	}

	if err := listener.Close(); err != nil {
		return false
	}

	return true
}

func GetAvailablePort() int {
	port := 3000

	for !isPortAvailable(port) {
		port += 1
	}

	return port
}
