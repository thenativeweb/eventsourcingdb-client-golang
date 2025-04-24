package eventsourcingdb

import "net/url"

func (c *Client) getURL(path string) (*url.URL, error) {
	urlPath, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	targetURL := c.baseURL.ResolveReference(urlPath)

	return targetURL, nil
}
