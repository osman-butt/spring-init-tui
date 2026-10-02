// Package initializr talks to a Spring Initializr instance such as
// https://start.spring.io.
package initializr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		defer resp.Body.Close()
		return nil, statusError(resp)
	}
	return resp, nil
}

// statusError includes the "message" field Initializr sends with error
// responses, e.g. "Unknown dependency 'x' check project metadata".
func statusError(resp *http.Response) error {
	var body struct {
		Message string `json:"message"`
	}
	// A body that is not JSON leaves the message empty, which is fine.
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body)
	if body.Message != "" {
		return fmt.Errorf("unexpected status %s: %s", resp.Status, body.Message)
	}
	return fmt.Errorf("unexpected status %s", resp.Status)
}
