package network

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SPDG/unicli/internal/client"
)

func testAPI(t *testing.T, h http.Handler) *API {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := client.New(srv.URL, "k", false)
	if err != nil {
		t.Fatal(err)
	}
	return New(c)
}

func TestInfoAndSites(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/info", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Info{ApplicationVersion: "10.5.67"})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("limit=%s", r.URL.Query().Get("limit"))
		}
		_ = json.NewEncoder(w).Encode(Page[Site]{
			Offset: 0, Limit: 10, Count: 1, TotalCount: 1,
			Data: []Site{{ID: "site-1", InternalRef: "default", Name: "leska"}},
		})
	})
	api := testAPI(t, mux)
	info, err := api.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.ApplicationVersion != "10.5.67" {
		t.Fatalf("version=%s", info.ApplicationVersion)
	}
	sites, err := api.Sites(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites.Data) != 1 || sites.Data[0].Name != "leska" {
		t.Fatalf("%+v", sites)
	}
}

func TestDevicesClientsAndRestart(t *testing.T) {
	var restartBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/devices", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[Device]{
			Count: 1, TotalCount: 1, Limit: 25,
			Data: []Device{{ID: "dev-1", Name: "UDM", State: "ONLINE"}},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/devices/dev-1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Device{ID: "dev-1", Name: "UDM", Model: "UDM Pro"})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/devices/dev-1/statistics/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(DeviceStatistics{UptimeSec: 42, CPUUtilizationPct: 1.5})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/devices/dev-1/actions", func(w http.ResponseWriter, r *http.Request) {
		restartBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/clients", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[Client]{
			Count: 1, TotalCount: 1,
			Data: []Client{{ID: "c1", Name: "pi", IPAddress: "192.168.5.221"}},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/interfaces/ports/5/actions", func(w http.ResponseWriter, r *http.Request) {
		t.Error("wrong port path")
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/devices/dev-1/interfaces/ports/5/actions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	api := testAPI(t, mux)
	ctx := context.Background()
	devs, err := api.Devices(ctx, "site-1", 0, 25)
	if err != nil || len(devs.Data) != 1 {
		t.Fatalf("%v %+v", err, devs)
	}
	dev, err := api.Device(ctx, "site-1", "dev-1")
	if err != nil || dev.Model != "UDM Pro" {
		t.Fatalf("%v %+v", err, dev)
	}
	stats, err := api.DeviceStatistics(ctx, "site-1", "dev-1")
	if err != nil || stats.UptimeSec != 42 {
		t.Fatalf("%v %+v", err, stats)
	}
	if err := api.RestartDevice(ctx, "site-1", "dev-1"); err != nil {
		t.Fatal(err)
	}
	if string(restartBody) != `{"action":"RESTART"}` {
		t.Fatalf("restart body=%s", restartBody)
	}
	if err := api.PowerCyclePort(ctx, "site-1", "dev-1", 5); err != nil {
		t.Fatal(err)
	}
	clients, err := api.Clients(ctx, "site-1", 0, 25)
	if err != nil || clients.Data[0].Name != "pi" {
		t.Fatalf("%v %+v", err, clients)
	}
}

func TestPageQueryDefaults(t *testing.T) {
	q := pageQuery(-1, 0)
	if q.Get("offset") != "0" || q.Get("limit") != "25" {
		t.Fatalf("%v", q)
	}
}
