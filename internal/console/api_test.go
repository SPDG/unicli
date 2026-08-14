package console

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SPDG/unicli/internal/client"
)

func TestSystemAndProbeApps(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "lab", "deviceState": "CONNECTED", "hasInternet": true,
			"hardware": map[string]any{"shortname": "UDMPRO"},
		})
	})
	mux.HandleFunc("/api/apps", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"apps": []any{}, "controllers": []any{}})
	})
	mux.HandleFunc("/proxy/network/integration/v1/info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"applicationVersion":"10.0.0"}`))
	})
	mux.HandleFunc("/proxy/protect/integration/v1/meta/info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"applicationVersion":"7.0.0"}`))
	})
	mux.HandleFunc("/proxy/access/api/v2/doors", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"CODE_NOT_FOUND","msg":"no-man zone"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := client.New(srv.URL, "k", false)
	if err != nil {
		t.Fatal(err)
	}
	api := New(c)
	sys, err := api.System(context.Background())
	if err != nil || sys["name"] != "lab" {
		t.Fatalf("%v %+v", err, sys)
	}
	apps := api.ProbeApps(context.Background())
	if len(apps) != 3 {
		t.Fatalf("%+v", apps)
	}
	if !apps[0].Available || apps[0].Version != "10.0.0" {
		t.Fatalf("network %+v", apps[0])
	}
	if !apps[1].Available || apps[1].Version != "7.0.0" {
		t.Fatalf("protect %+v", apps[1])
	}
	if apps[2].Available {
		t.Fatalf("access should be down %+v", apps[2])
	}
}
