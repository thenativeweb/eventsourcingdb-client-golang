package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type RegisterHandlers func(mux *http.ServeMux)

func NewHTTPServer(registerHandlers RegisterHandlers) (httpAddress string, cancel context.CancelFunc) {
	port := GetAvailablePort()
	localServerAddress := fmt.Sprintf("localhost:%d", port)

	mux := http.NewServeMux()
	registerHandlers(mux)

	mux.HandleFunc("/__test__/ready", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		localServer := &http.Server{
			Addr:    localServerAddress,
			Handler: mux,
			BaseContext: func(listener net.Listener) context.Context {
				return ctx
			},
		}

		_ = localServer.ListenAndServe()
	}()

	serverAddress := fmt.Sprintf("http://%s", localServerAddress)

	var retryError error
	for try := 0; try < 10; try++ {
		time.Sleep(100 * time.Millisecond)
		response, err := http.Get(fmt.Sprintf("%s/__test__/ready", serverAddress))
		if err != nil {
			retryError = errors.Join(retryError, err)
			continue
		}

		if response.StatusCode != http.StatusOK {
			retryError = errors.Join(retryError, err)
			continue
		}

		retryError = nil
		break
	}
	if retryError != nil {
		panic(retryError)
	}

	return serverAddress, cancel
}
