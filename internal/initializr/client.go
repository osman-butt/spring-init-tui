// Package initializr talks to a Spring Initializr instance such as
// https://start.spring.io.
package initializr

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	DefaultBaseURL = "https://start.spring.io"

	userAgent = "spring-init-tui"
)

// Client is a Spring Initializr API client.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a client for https://start.spring.io.
func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) get(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return resp, nil
}
