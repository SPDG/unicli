package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetJSONSendsAPIKeyAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "secret" {
			t.Errorf("missing API key header: %q", r.Header.Get("X-API-KEY"))
		}
		if r.URL.Path != "/proxy/network/integration/v1/info" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"applicationVersion": "10.5.67"})
	}))
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		ApplicationVersion string `json:"applicationVersion"`
	}
	if err := c.GetJSON(context.Background(), "network", "info", nil, &out); err != nil {
		t.Fatal(err)
	}
	if out.ApplicationVersion != "10.5.67" {
		t.Fatalf("got %q", out.ApplicationVersion)
	}
}

func TestPostJSONWritesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if strings.TrimSpace(string(body)) != `{"action":"RESTART"}` {
			t.Errorf("body = %s", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, "k", false)
	if err != nil {
		t.Fatal(err)
	}
	err = c.PostJSON(context.Background(), "network", "sites/s/devices/d/actions", map[string]string{"action": "RESTART"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAPIErrorOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, "k", false)
	if err != nil {
		t.Fatal(err)
	}
	err = c.GetJSON(context.Background(), "network", "info", nil, &map[string]any{})
	var ae APIError
	if !errors.As(err, &ae) {
		t.Fatalf("want APIError, got %T %v", err, err)
	}
	if ae.Status != 401 {
		t.Fatalf("status = %d", ae.Status)
	}
}

func TestHTMLResponseIsAppUnavailable(t *testing.T) {
	html := `<!doctype html><html lang="en"><head><title>UniFi OS</title></head><body></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, "k", false)
	if err != nil {
		t.Fatal(err)
	}
	err = c.GetPathJSON(context.Background(), "/proxy/access/api/v2/doors", nil, &json.RawMessage{})
	var ue AppUnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("want AppUnavailableError, got %T %v", err, err)
	}
	if ue.Status != 200 {
		t.Fatalf("status = %d", ue.Status)
	}
}

func TestNewRejectsEmptyHost(t *testing.T) {
	_, err := New("", "k", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLooksLikeHTML(t *testing.T) {
	if !looksLikeHTML([]byte("  <!DOCTYPE html>")) {
		t.Fatal("doctype")
	}
	if !looksLikeHTML([]byte("<html>")) {
		t.Fatal("html")
	}
	if looksLikeHTML([]byte(`{"ok":true}`)) {
		t.Fatal("json should not look like HTML")
	}
}
