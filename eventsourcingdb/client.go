package eventsourcingdb

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/configuration"
	internalhttputil "github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
)

type Client struct {
	configuration configuration.ClientConfiguration
}

func NewClient(baseURL string, accessToken string) (Client, error) {
	if strconv.IntSize != 64 {
		return Client{}, NewClientError("64-bit architecture required")
	}
	if accessToken == "" {
		return Client{}, NewInvalidArgumentError("accessToken", "must not be empty")
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return Client{}, NewInvalidArgumentError("baseURL", err.Error())
	}
	if parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https" {
		return Client{}, NewInvalidArgumentError("baseURL", "must use HTTP or HTTPS")
	}

	clientConfiguration := configuration.GetDefaultConfiguration(parsedBaseURL, accessToken)

	client := Client{clientConfiguration}

	return client, nil
}

func (client Client) requestServer(method string, path string, body io.Reader) (*http.Response, error) {
	routeURL := client.configuration.BaseURL.JoinPath(path)
	request, err := http.NewRequest(method, routeURL.String(), body)
	if err != nil {
		return nil, NewInternalError(err)
	}

	request.Header.Add("X-EventSourcingDB-Protocol-Version", client.configuration.ProtocolVersion.String())
	request.Header.Add("Authorization", "Bearer "+client.configuration.AccessToken)

	httpClient := &http.Client{}
	var response *http.Response

	response, requestError := httpClient.Do(request)
	if requestError != nil {
		return response, NewServerError(requestError.Error())
	}
	if clientError := internalhttputil.ValidateProtocolVersion(response, client.configuration.ProtocolVersion); clientError != nil {
		return response, NewClientError(clientError.Error())
	}

	if response.StatusCode != http.StatusOK {
		errorMessage := response.Status
		if responseBody, err := io.ReadAll(response.Body); err == nil {
			errorMessage += ": " + string(responseBody)
		}

		if internalhttputil.IsClientError(response) {
			return response, NewClientError(errorMessage)
		}
		if internalhttputil.IsServerError(response) {
			return response, NewServerError(errorMessage)
		}

		return response, NewServerError(fmt.Sprintf("unexpected response status: %s", response.Status))
	}

	return response, nil
}
