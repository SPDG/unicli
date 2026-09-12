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
	if !strings.Contains(out, "network traffic wan") || !strings.Contains(out, "network traffic clients") {
		t.Fatal(out)
	}
	if !strings.Contains(out, network.TrafficSchemaWAN) {
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
