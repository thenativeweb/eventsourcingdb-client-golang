package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/thenativeweb/goutils/v2/coreutils/retry"
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

	err := retry.WithBackoff(ctx, 10, func() error {
		response, err := http.Get(fmt.Sprintf("%s/__test__/ready", serverAddress))
		if err != nil {
			return err
		}

		if response.StatusCode != http.StatusOK {
			return errors.New("server not ready")
		}

		return nil
	})
	if err != nil {
		panic(err)
	}

	return serverAddress, cancel
}
