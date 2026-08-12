package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a UniFi OS console using local Integration APIs.
type Client struct {
	base       *url.URL
	apiKey     string
	httpClient *http.Client
}

func New(host, apiKey string, insecure bool) (*Client, error) {
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse host: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid host %q", host)
	}
	// Keep only scheme + host (+ port); path from config is ignored.
	base := &url.URL{Scheme: u.Scheme, Host: u.Host}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec // UniFi lab certs are commonly self-signed
	return &Client{
		base:   base,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: tr,
		},
	}, nil
}

func (c *Client) GetJSON(ctx context.Context, app, path string, query url.Values, dest any) error {
	body, status, err := c.do(ctx, http.MethodGet, app, path, query, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return APIError{Status: status, Body: body}
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, app, path string, query url.Values, reqBody io.Reader) ([]byte, int, error) {
	path = strings.TrimPrefix(path, "/")
	u := c.base.ResolveReference(&url.URL{
		Path: "/proxy/" + app + "/integration/v1/" + path,
	})
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// APIError is a non-2xx response from the console.
type APIError struct {
	Status int
	Body   []byte
}

func (e APIError) Error() string {
	msg := strings.TrimSpace(string(e.Body))
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	if msg == "" {
		return fmt.Sprintf("API error: HTTP %d", e.Status)
	}
	return fmt.Sprintf("API error: HTTP %d: %s", e.Status, msg)
}
