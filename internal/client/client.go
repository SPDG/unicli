package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client talks to a UniFi OS console using local APIs.
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
	return c.roundTrip(ctx, http.MethodGet, integrationPath(app, path), query, nil, dest)
}

// GetBytes GETs an Integration path and returns the raw body (snapshots, thumbnails).
func (c *Client) GetBytes(ctx context.Context, app, path string, query url.Values) ([]byte, error) {
	raw, status, err := c.do(ctx, http.MethodGet, integrationPath(app, path), query, nil)
	if err != nil {
		return nil, err
	}
	if looksLikeHTML(raw) {
		return nil, AppUnavailableError{Path: integrationPath(app, path), Status: status}
	}
	if status < 200 || status >= 300 {
		return nil, APIError{Status: status, Body: raw}
	}
	return raw, nil
}

func (c *Client) PostJSON(ctx context.Context, app, path string, payload any, dest any) error {
	return c.writeJSON(ctx, http.MethodPost, integrationPath(app, path), payload, dest)
}

func (c *Client) PutJSON(ctx context.Context, absPath string, payload any, dest any) error {
	return c.writeJSON(ctx, http.MethodPut, absPath, payload, dest)
}

func (c *Client) PutAppJSON(ctx context.Context, app, path string, payload any, dest any) error {
	return c.writeJSON(ctx, http.MethodPut, integrationPath(app, path), payload, dest)
}

func (c *Client) PatchJSON(ctx context.Context, app, path string, payload any, dest any) error {
	return c.writeJSON(ctx, http.MethodPatch, integrationPath(app, path), payload, dest)
}

func (c *Client) DeleteJSON(ctx context.Context, app, path string, query url.Values) error {
	return c.roundTrip(ctx, http.MethodDelete, integrationPath(app, path), query, nil, nil)
}

// GetPathJSON GETs an absolute path on the console (e.g. /proxy/access/api/v2/doors).
func (c *Client) GetPathJSON(ctx context.Context, absPath string, query url.Values, dest any) error {
	return c.roundTrip(ctx, http.MethodGet, absPath, query, nil, dest)
}

func (c *Client) PostPathJSON(ctx context.Context, absPath string, payload any, dest any) error {
	return c.writeJSON(ctx, http.MethodPost, absPath, payload, dest)
}

func (c *Client) DeletePathJSON(ctx context.Context, absPath string, query url.Values) error {
	return c.roundTrip(ctx, http.MethodDelete, absPath, query, nil, nil)
}

func (c *Client) writeJSON(ctx context.Context, method, absPath string, payload any, dest any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	return c.roundTrip(ctx, method, absPath, nil, body, dest)
}

func integrationPath(app, path string) string {
	path = strings.TrimPrefix(path, "/")
	return "/proxy/" + app + "/integration/v1/" + path
}

func (c *Client) roundTrip(ctx context.Context, method, absPath string, query url.Values, reqBody io.Reader, dest any) error {
	raw, status, err := c.do(ctx, method, absPath, query, reqBody)
	if err != nil {
		return err
	}
	if looksLikeHTML(raw) {
		return AppUnavailableError{Path: absPath, Status: status}
	}
	if status < 200 || status >= 300 {
		return APIError{Status: status, Body: raw}
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		if looksLikeHTML(raw) {
			return AppUnavailableError{Path: absPath, Status: status}
		}
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, absPath string, query url.Values, reqBody io.Reader) ([]byte, int, error) {
	if !strings.HasPrefix(absPath, "/") {
		absPath = "/" + absPath
	}
	u := c.base.ResolveReference(&url.URL{Path: absPath})
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var payload []byte
	if reqBody != nil {
		var err error
		payload, err = io.ReadAll(reqBody)
		if err != nil {
			return nil, 0, err
		}
	}

	var lastBody []byte
	var lastStatus int
	for attempt := 0; attempt < 4; attempt++ {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("X-API-KEY", c.apiKey)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, resp.StatusCode, err
		}
		lastBody, lastStatus = raw, resp.StatusCode
		if resp.StatusCode != http.StatusTooManyRequests || attempt == 3 {
			return raw, resp.StatusCode, nil
		}
		wait := retryWait(resp.Header.Get("Retry-After"), raw)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, 0, ctx.Err()
		case <-timer.C:
		}
	}
	return lastBody, lastStatus, nil
}

func retryWait(retryAfter string, body []byte) time.Duration {
	if n, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && n >= 0 {
		d := time.Duration(n) * time.Second
		if d > 10*time.Second {
			d = 10 * time.Second
		}
		return d
	}
	var env struct {
		WindowMs int `json:"windowMs"`
	}
	if json.Unmarshal(body, &env) == nil && env.WindowMs > 0 {
		d := time.Duration(env.WindowMs) * time.Millisecond
		if d > 10*time.Second {
			d = 10 * time.Second
		}
		return d
	}
	return time.Second
}

func looksLikeHTML(body []byte) bool {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "<!doctype html") ||
		strings.HasPrefix(lower, "<html") ||
		strings.Contains(lower[:min(200, len(lower))], "<html")
}

// APIError is a non-2xx JSON/API response from the console.
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

// AppUnavailableError means the console returned the UniFi OS HTML shell instead of an app API.
type AppUnavailableError struct {
	Path   string
	Status int
}

func (e AppUnavailableError) Error() string {
	return fmt.Sprintf("application unavailable at %s (console returned HTML, HTTP %d)", e.Path, e.Status)
}
