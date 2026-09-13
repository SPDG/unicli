package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
)

func trafficMock(t *testing.T, report http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"applicationVersion":"10.6.101"}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"site-1","internalReference":"default","name":"lab"}]}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/clients", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":100,"count":1,"totalCount":1,"data":[{"id":"c2","name":"laptop","macAddress":"aa:00:00:00:00:02"}]}`))
	})
	mux.HandleFunc("/proxy/network/api/s/default/stat/sysinfo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"timezone":"Europe/Warsaw","version":"10.6.101"}]}`))
	})
	mux.HandleFunc("/proxy/network/api/s/default/rest/user", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"_id":"id-nas","mac":"aa:00:00:00:00:01","name":"nas"},{"_id":"id-pc","mac":"aa:00:00:00:00:02","name":"laptop"}]}`))
	})
	mux.HandleFunc("/proxy/network/api/s/other/stat/report/", func(w http.ResponseWriter, r *http.Request) {
		t.Error("other site")
		http.Error(w, "wrong site", 500)
	})
	mux.HandleFunc("/proxy/network/api/s/default/stat/report/", report)
	mux.HandleFunc("/proxy/network/v2/api/site/default/traffic", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("internet method=%s", r.Method)
		}
		if strings.Contains(r.URL.EscapedPath(), "%3F") {
			t.Errorf("query encoded into path: %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_usage_by_app": []map[string]any{
				{"application": 65535, "category": 255, "bytes_received": 1000, "bytes_transmitted": 100, "total_bytes": 1100, "client_count": 2},
				{"application": 185, "category": 20, "bytes_received": 400, "bytes_transmitted": 50, "total_bytes": 450, "client_count": 1},
			},
			"client_usage_by_app": []map[string]any{
				{
					"client": map[string]any{"mac": "aa:00:00:00:00:01", "name": "nas", "is_wired": true},
					"usage_by_app": []map[string]any{
						{"application": 65535, "bytes_received": 500, "bytes_transmitted": 50, "total_bytes": 550},
						{"application": 185, "bytes_received": 400, "bytes_transmitted": 50, "total_bytes": 450},
					},
				},
				{
					"client": map[string]any{"mac": "aa:00:00:00:00:02", "name": "laptop", "is_wired": false},
					"usage_by_app": []map[string]any{
						{"application": 65535, "bytes_received": 500, "bytes_transmitted": 50, "total_bytes": 550},
					},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestTrafficWANAndClientsCLI(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	srv := trafficMock(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(strings.ToLower(string(body)), "api") && strings.Contains(string(body), "key") {
			t.Errorf("body looks like a secret: %s", body)
		}
		day := timeUnix(t, "2026-09-01T00:00:00+02:00")
		if strings.Contains(r.URL.Path, ".user") {
			_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"o":"user","user":"aa:00:00:00:00:01","oid":"aa:00:00:00:00:01","time":` + day + `,"rx_bytes":8000,"tx_bytes":100},{"o":"user","user":"aa:00:00:00:00:02","oid":"aa:00:00:00:00:02","time":` + day + `,"rx_bytes":1000,"tx_bytes":200}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"o":"site","time":` + day + `,"wan-rx_bytes":500,"wan-tx_bytes":40}]}`))
	})
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "network", "traffic", "wan", "--json",
		"--start", "2026-09-01", "--end", "2026-09-01", "--timezone", "Europe/Warsaw")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if strings.Contains(out, "k") && strings.Contains(out, "API") {
		t.Fatalf("key leaked: %s", out)
	}
	var wan network.WANTrafficReport
	if err := json.Unmarshal([]byte(out), &wan); err != nil {
		t.Fatal(err)
	}
	if wan.Schema != network.TrafficSchemaWAN || wan.Scope != "wan" || wan.Totals == nil || wan.Totals.DownloadBytes != 500 {
		t.Fatalf("%s", out)
	}

	out, _, err = execCLI(t, "network", "traffic", "clients", "--json",
		"--start", "2026-09-01", "--end", "2026-09-01", "--timezone", "Europe/Warsaw",
		"--sort", "total", "--top", "10")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	var clients network.ClientTrafficReport
	if err := json.Unmarshal([]byte(out), &clients); err != nil {
		t.Fatal(err)
	}
	if clients.Scope != "client-all-traffic" || len(clients.Clients) != 2 {
		t.Fatalf("%s", out)
	}
	if clients.Clients[0].MAC != "aa:00:00:00:00:01" || clients.Clients[0].Connected == nil || *clients.Clients[0].Connected {
		t.Fatalf("offline client should rank: %s", out)
	}
	if clients.Ranking.Partial {
		t.Fatalf("unexpected partial: %+v", clients.Ranking)
	}

	out, _, err = execCLI(t, "network", "traffic", "internet", "--json",
		"--start", "2026-09-01", "--end", "2026-09-01", "--timezone", "Europe/Warsaw",
		"--sort", "total", "--top", "10")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	var internet network.InternetTrafficReport
	if err := json.Unmarshal([]byte(out), &internet); err != nil {
		t.Fatal(err)
	}
	if internet.Schema != network.TrafficSchemaInternet || internet.Scope != "internet" || internet.Totals == nil {
		t.Fatalf("%s", out)
	}
	if internet.Totals.DownloadBytes != 1400 || internet.Unidentified == nil || internet.Unidentified.TotalBytes == internet.Totals.TotalBytes {
		t.Fatalf("unidentified must not be the site total: %s", out)
	}
	if len(internet.Clients) != 2 || internet.Clients[0].MAC != "aa:00:00:00:00:01" {
		t.Fatalf("%s", out)
	}

	out, _, err = execCLI(t, "network", "traffic", "clients", "--json",
		"--start", "2026-09-01", "--end", "2026-09-01", "--timezone", "Europe/Warsaw",
		"--sort", "download", "--top", "1")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if err := json.Unmarshal([]byte(out), &clients); err != nil {
		t.Fatal(err)
	}
	if !clients.Ranking.Partial || clients.Ranking.Returned != 1 || clients.Ranking.RankedClients != 2 {
		t.Fatalf("%+v", clients.Ranking)
	}
}

func TestTrafficDeniedUnsupportedAndUsage(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	srv := trafficMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"meta":{"rc":"error"}}`))
	})
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")
	out, _, err := execCLI(t, "network", "traffic", "wan", "--json", "--start", "2026-09-01", "--end", "2026-09-02", "--timezone", "UTC")
	if exitCode(err) != exitcode.Permission {
		t.Fatalf("code=%d err=%v out=%s", exitCode(err), err, out)
	}
	if !strings.Contains(out, `"status": "denied"`) || !strings.Contains(out, `"totals": null`) {
		t.Fatalf("denied json: %s", out)
	}

	srv = trafficMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"missing"}`))
	})
	t.Setenv("UNIFI_HOST", srv.URL)
	out, _, err = execCLI(t, "network", "traffic", "wan", "--json", "--start", "2026-09-01", "--end", "2026-09-02", "--timezone", "UTC")
	if exitCode(err) != exitcode.Unsupported {
		t.Fatalf("code=%d err=%v out=%s", exitCode(err), err, out)
	}
	if !strings.Contains(out, `"status": "unsupported"`) {
		t.Fatalf("%s", out)
	}

	_, _, err = execCLI(t, "network", "traffic", "clients", "--json", "--sort", "nope")
	if exitCode(err) != exitcode.Usage {
		t.Fatalf("sort code=%d err=%v", exitCode(err), err)
	}
}

func TestSchemaAdvertisesTraffic(t *testing.T) {
	out, _, err := execCLI(t, "schema", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "network traffic wan") || !strings.Contains(out, "network traffic clients") || !strings.Contains(out, "network traffic internet") {
		t.Fatal(out)
	}
	if !strings.Contains(out, network.TrafficSchemaWAN) || !strings.Contains(out, network.TrafficSchemaInternet) {
		t.Fatal(out)
	}
}

func timeUnix(t *testing.T, rfc3339 string) string {
	t.Helper()
	tm, err := jsonTime(rfc3339)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func jsonTime(rfc3339 string) (string, error) {
	var n int64
	tt, err := parseRFC(rfc3339)
	if err != nil {
		return "", err
	}
	n = tt
	b, _ := json.Marshal(n)
	return string(b), nil
}

func parseRFC(s string) (int64, error) {
	t, err := network.ParseTrafficTime(s, nil, false)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}
