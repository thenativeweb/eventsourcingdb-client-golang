package httputil

import (
	"context"
	"fmt"
	"io"
	"net/http"

	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/configuration"
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

	// TODO: evalute if we can remove context
	executor := func(ctx context.Context) (*http.Response, error) {
		var response *http.Response

		response, requestError := httpClient.Do(request)
		if requestError != nil {
			return response, customErrors.NewServerError(requestError.Error())
		}
		if clientError := validateProtocolVersion(response, factory.configuration.ProtocolVersion); clientError != nil {
			return response, customErrors.NewClientError(clientError.Error())
		}

		if response.StatusCode != http.StatusOK {
			errorMessage := response.Status
			if responseBody, err := io.ReadAll(response.Body); err == nil {
				errorMessage += ": " + string(responseBody)
			}

			if IsClientError(response) {
				return response, customErrors.NewClientError(errorMessage)
			}
			if IsServerError(response) {
				return response, customErrors.NewServerError(errorMessage)
			}

			return response, customErrors.NewServerError(fmt.Sprintf("unexpected response status: %s", response.Status))
		}

		return response, nil
	}

	return executor, nil
}
