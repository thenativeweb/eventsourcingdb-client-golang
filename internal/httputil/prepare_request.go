package httputil

import (
	"context"
	"errors"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/configuration"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/goutils/v2/coreutils/contextutils"
	"github.com/thenativeweb/goutils/v2/coreutils/retry"
	"io"
	"net/http"
)

type RequestFactory struct {
	configuration configuration.ClientConfiguration
}

type RequestExecutor func(ctx context.Context) (*http.Response, error)

func NewRequestFactory(configuration configuration.ClientConfiguration) *RequestFactory {
	return &RequestFactory{
		configuration: configuration,
	}
}

func (factory RequestFactory) Create(method string, path string, body io.Reader) (RequestExecutor, error) {
	routeURL := factory.configuration.BaseURL.JoinPath(path)
	httpClient := &http.Client{}
	request, err := http.NewRequest(method, routeURL.String(), body)
	if err != nil {
		return nil, err
	}

	addProtocolVersion(request, factory.configuration.ProtocolVersion)
	addAccessToken(request, factory.configuration.AccessToken)

	executor := func(ctx context.Context) (*http.Response, error) {
		var clientError error
		var response *http.Response

		retryErr := retry.WithBackoff(ctx, factory.configuration.MaxTries, func() error {
			var requestError error
			response, requestError = httpClient.Do(request)
			if requestError != nil {
				return requestError
			}
			if err := validateProtocolVersion(response, factory.configuration.ProtocolVersion); err != nil {
				clientError = err

				// Do not retry if the protocol version is not supported.
				return nil
			}

			if response.StatusCode == http.StatusOK {
				return nil
			}

			errorMessage := response.Status
			if responseBody, err := io.ReadAll(response.Body); err == nil {
				errorMessage += ": " + string(responseBody)
			}

			if IsClientError(response) {
				clientError = errors.New(errorMessage)

				// Do not retry client errors.
				return nil
			}
			if IsServerError(response) {
				return errors.New(errorMessage)
			}

			return customErrors.NewServerError(fmt.Sprintf("unexpected response status: %s", response.Status))
		})
		if contextutils.IsContextTerminationError(retryErr) {
			return response, retryErr
		}
		if retryErr != nil {
			return response, customErrors.NewServerError(retryErr.Error())
		}
		if clientError != nil {
			return response, customErrors.NewClientError(clientError.Error())
		}

		return response, nil
	}

	return executor, nil
}
