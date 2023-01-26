package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

type RegisterHandlers func(mux *http.ServeMux)

func NewHTTPServer(registerHandlers RegisterHandlers) (httpAddress string, cancel context.CancelFunc) {
	port := GetFreePort()
	localServerAddress := fmt.Sprintf("localhost:%d", port)

	mux := http.NewServeMux()
	registerHandlers(mux)

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

	return fmt.Sprintf("http://%s", localServerAddress), cancel
}
