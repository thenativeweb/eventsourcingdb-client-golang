package httpserver

import (
	"fmt"
	"net"
)

func isPortFree(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))

	if err != nil {
		return false
	}

	if err := listener.Close(); err != nil {
		return false
	}

	return true
}

func GetFreePort() int {
	port := 3000

	for !isPortFree(port) {
		port += 1
	}

	return port
}
