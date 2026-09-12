package network

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SPDG/unicli/internal/client"
)

func TestWANHourlyQueryIsNarrowerThanDaily(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	observed := time.Date(2026, 9, 13, 1, 0, 0, 0, loc)
	starts := map[string]int64{}
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/api/s/default/stat/report/", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Start int64 `json:"start"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case strings.Contains(r.URL.Path, "daily."):
			starts["daily"] = body.Start
		case strings.Contains(r.URL.Path, "hourly."):
			starts["hourly"] = body.Start
		case strings.Contains(r.URL.Path, "5minutes."):
			starts["5minutes"] = body.Start
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"rc": "ok"}, "data": []any{}})
	})
	api := testAPI(t, mux)
	_, err := api.WANTraffic(context.Background(), TrafficSite{InternalReference: "default"}, TrafficQuery{
		Start: start, End: observed, Location: loc, Timezone: "UTC", ObservedAt: observed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if starts["daily"] != start.UnixMilli() {
		t.Fatalf("daily start=%d", starts["daily"])
	}
	if starts["hourly"] <= starts["daily"] {
		t.Fatalf("hourly should be narrower: %+v", starts)
	}
	if starts["5minutes"] <= starts["hourly"] {
		t.Fatalf("5min should be narrower than hourly: %+v", starts)
	}
}

func TestWANTrafficCrossSiteIsolation(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	observed := time.Date(2026, 9, 2, 0, 0, 0, 0, loc)
	var hitOther bool
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/api/s/other/stat/report/", func(w http.ResponseWriter, r *http.Request) {
		hitOther = true
		http.Error(w, "wrong site", 500)
	})
	mux.HandleFunc("/proxy/network/api/s/default/stat/report/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"mac"`) {
			t.Errorf("unexpected mac filter: %s", body)
		}
		day := start.UnixMilli()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]any{"rc": "ok"},
			"data": []map[string]any{{
				"o": "site", "time": day,
				"wan-rx_bytes": 100, "wan-tx_bytes": 10,
			}},
		})
	})
	api := testAPI(t, mux)
	rep, err := api.WANTraffic(context.Background(), TrafficSite{ID: "site-1", InternalReference: "default", Name: "lab"}, TrafficQuery{
		Start: start, End: observed, Location: loc, Timezone: "UTC", ObservedAt: observed, AppVersion: "10.6.101",
	})
	if err != nil {
		t.Fatal(err)
	}
	if hitOther {
		t.Fatal("queried a different site")
	}
	if rep.Totals == nil || rep.Totals.DownloadBytes != 100 || rep.Scope != trafficScopeWAN {
		t.Fatalf("%+v", rep.Totals)
	}
	if rep.Schema != TrafficSchemaWAN || rep.Backend != trafficBackendLegacy {
		t.Fatalf("%+v", rep)
	}
}

func TestClientTrafficRanksOfflineFromHistory(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	observed := time.Date(2026, 9, 2, 0, 0, 0, 0, loc)
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/api/s/default/stat/report/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ".user") {
			_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"rc": "ok"}, "data": []any{}})
			return
		}
		day := start.UnixMilli()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]any{"rc": "ok"},
			"data": []map[string]any{
				{"o": "user", "user": "aa:00:00:00:00:01", "oid": "aa:00:00:00:00:01", "time": day, "rx_bytes": 8000, "tx_bytes": 100},
				{"o": "user", "user": "aa:00:00:00:00:02", "oid": "aa:00:00:00:00:02", "time": day, "rx_bytes": 1000, "tx_bytes": 200},
			},
		})
	})
	mux.HandleFunc("/proxy/network/api/s/default/rest/user", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]any{"rc": "ok"},
			"data": []map[string]any{
				{"_id": "id-nas", "mac": "aa:00:00:00:00:01", "name": "nas", "hostname": "nas.lab"},
				{"_id": "id-pc", "mac": "aa:00:00:00:00:02", "name": "laptop"},
			},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/clients", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[Client]{
			Count: 1, TotalCount: 1, Limit: 100,
			Data: []Client{{ID: "c2", Name: "laptop", MacAddress: "aa:00:00:00:00:02"}},
		})
	})
	api := testAPI(t, mux)
	rep, err := api.ClientTraffic(context.Background(), TrafficSite{ID: "site-1", InternalReference: "default", Name: "lab"}, TrafficQuery{
		Start: start, End: observed, Location: loc, Timezone: "UTC", ObservedAt: observed,
		Sort: trafficSortTotal, Top: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Scope != trafficScopeClientAll {
		t.Fatalf("scope=%s", rep.Scope)
	}
	if len(rep.Clients) != 2 {
		t.Fatalf("clients=%d", len(rep.Clients))
	}
	if rep.Clients[0].MAC != "aa:00:00:00:00:01" || rep.Clients[0].Connected == nil || *rep.Clients[0].Connected {
		t.Fatalf("offline nas should win: %+v", rep.Clients[0])
	}
	if rep.Clients[0].Name != "nas" {
		t.Fatalf("name %+v", rep.Clients[0])
	}
}

func TestTrafficReportDeniedAndUnsupported(t *testing.T) {
	denied := httptestHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"meta":{"rc":"error","msg":"forbidden"}}`))
	})
	api := testAPI(t, denied)
	_, err := api.StatReport(context.Background(), "default", "daily", "site", 1, 2, trafficWANAttrs)
	if !IsTrafficPermission(err) {
		t.Fatalf("denied: %v", err)
	}

	missing := httptestHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})
	api = testAPI(t, missing)
	_, err = api.StatReport(context.Background(), "default", "daily", "site", 1, 2, trafficWANAttrs)
	if !IsTrafficUnsupported(err) {
		t.Fatalf("unsupported: %v", err)
	}

	html := httptestHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!doctype html><html><title>UniFi OS</title></html>`))
	})
	api = testAPI(t, html)
	_, err = api.StatReport(context.Background(), "default", "daily", "site", 1, 2, trafficWANAttrs)
	if !IsTrafficUnsupported(err) {
		t.Fatalf("html: %v", err)
	}
}

func TestWANTrafficEmptyIsNotZero(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	observed := time.Date(2026, 9, 2, 0, 0, 0, 0, loc)
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/api/s/default/stat/report/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"rc": "ok"}, "data": []any{}})
	})
	api := testAPI(t, mux)
	rep, err := api.WANTraffic(context.Background(), TrafficSite{InternalReference: "default"}, TrafficQuery{
		Start: start, End: observed, Location: loc, Timezone: "UTC", ObservedAt: observed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Totals != nil {
		t.Fatalf("empty reports must not become zero totals: %+v", rep.Totals)
	}
	if rep.Coverage.Status != "empty" {
		t.Fatalf("coverage=%+v", rep.Coverage)
	}
}

func httptestHandler(h http.HandlerFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h)
	return mux
}

func TestIsTrafficPermissionValueError(t *testing.T) {
	err := client.APIError{Status: 403, Body: []byte("no")}
	if !IsTrafficPermission(err) {
		t.Fatal(err)
	}
}
