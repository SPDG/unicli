package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SPDG/unicli/internal/client"
	"github.com/SPDG/unicli/internal/exitcode"
)

func execCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := NewRoot()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errb.String(), err
}

func exitCode(err error) int {
	var xe *ExitError
	if errors.As(err, &xe) {
		return xe.Code
	}
	if err == nil {
		return 0
	}
	return -1
}

func clearUnifiEnv(t *testing.T) {
	t.Helper()
	t.Setenv("UNIFI_HOST", "")
	t.Setenv("UNIFI_API_KEY", "")
	t.Setenv("UNIFI_PROFILE", "")
	t.Setenv("UNIFI_INSECURE", "")
	t.Setenv("UNIFI_SITE", "")
}

func TestVersionAndSchemaJSON(t *testing.T) {
	out, _, err := execCLI(t, "version", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var ver map[string]string
	if err := json.Unmarshal([]byte(out), &ver); err != nil || ver["version"] == "" {
		t.Fatalf("%v %s", err, out)
	}

	out, _, err = execCLI(t, "schema", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(out), &schema); err != nil {
		t.Fatal(err)
	}
	codes := schema["exit_codes"].(map[string]any)
	if int(codes["mutation_blocked"].(float64)) != exitcode.MutationBlocked {
		t.Fatalf("exit codes: %v", codes)
	}
	cmds := schema["commands"].([]any)
	if len(cmds) < 10 {
		t.Fatalf("too few commands: %d", len(cmds))
	}
}

func TestMutationBlockedWithoutFlag(t *testing.T) {
	rootOpts = rootOptions{}
	err := requireMutations("network devices restart")
	if exitCode(err) != exitcode.MutationBlocked {
		t.Fatalf("code=%d err=%v", exitCode(err), err)
	}
	rootOpts.allowMutations = true
	if err := requireMutations("network devices restart"); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmRequiresYesWhenNonTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		_ = r.Close()
		_ = w.Close()
	})

	rootOpts = rootOptions{allowMutations: true, yes: false}
	err = requireConfirm("Restart device?")
	if exitCode(err) != exitcode.InputRequired {
		t.Fatalf("code=%d err=%v", exitCode(err), err)
	}
	rootOpts.yes = true
	if err := requireConfirm("Restart device?"); err != nil {
		t.Fatal(err)
	}
}

func TestMapAPIErr(t *testing.T) {
	if exitCode(mapAPIErr(client.AppUnavailableError{Path: "/x", Status: 200})) != exitcode.Unsupported {
		t.Fatal("unavailable")
	}
	if exitCode(mapAPIErr(client.APIError{Status: 401})) != exitcode.AuthRequired {
		t.Fatal("401")
	}
	if exitCode(mapAPIErr(client.APIError{Status: 404})) != exitcode.NotFound {
		t.Fatal("404")
	}
	if exitCode(mapAPIErr(client.APIError{Status: 429})) != exitcode.RateLimited {
		t.Fatal("429")
	}
	if exitCode(mapAPIErr(client.APIError{Status: 500})) != exitcode.Retryable {
		t.Fatal("500")
	}
}

func TestReadAPIKeyFromStdin(t *testing.T) {
	key, err := readAPIKeyFromStdin(strings.NewReader("  abc\n"))
	if err != nil || key != "abc" {
		t.Fatalf("%v %q", err, key)
	}
	_, err = readAPIKeyFromStdin(strings.NewReader("  \n"))
	if exitCode(err) != exitcode.InputRequired {
		t.Fatalf("empty key: %v", err)
	}
}

func TestProfileListAndUse(t *testing.T) {
	clearUnifiEnv(t)
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte("current: home\nprofiles:\n  home:\n    host: https://192.168.1.1\n    insecure: true\n  office:\n    host: https://office.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := execCLI(t, "--config", cfg, "profile", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"home"`) || !strings.Contains(out, `"office"`) {
		t.Fatal(out)
	}
	_, _, err = execCLI(t, "--config", cfg, "profile", "use", "office")
	if err != nil {
		t.Fatal(err)
	}
	out, _, err = execCLI(t, "--config", cfg, "profile", "show", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"office"`) {
		t.Fatal(out)
	}
}

func TestDoctorAgainstMockConsole(t *testing.T) {
	clearUnifiEnv(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"applicationVersion":"10.0.0"}`))
	})
	mux.HandleFunc("/proxy/protect/integration/v1/meta/info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"applicationVersion":"7.0.0"}`))
	})
	mux.HandleFunc("/proxy/access/api/v2/doors", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><title>UniFi OS</title></html>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "test-key")

	out, _, err := execCLI(t, "doctor", "--json")
	if err != nil {
		t.Fatalf("%v stdout=%s", err, out)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report["ok"] != true {
		t.Fatalf("%v", report)
	}
	if report["network_version"] != "10.0.0" || report["protect_version"] != "7.0.0" {
		t.Fatalf("%v", report)
	}
	if report["access_available"] != false {
		t.Fatalf("access should be unavailable: %v", report)
	}
}

func TestAccessInfoUnsupported(t *testing.T) {
	clearUnifiEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html></html>`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")
	_, _, err := execCLI(t, "access", "info", "--json")
	if exitCode(err) != exitcode.Unsupported {
		t.Fatalf("code=%d err=%v", exitCode(err), err)
	}
}

func TestNetworkDevicesListMock(t *testing.T) {
	clearUnifiEnv(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"site-1","internalReference":"default","name":"lab"}]}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/devices", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"dev-1","name":"UDM","model":"UDM Pro","state":"ONLINE"}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "network", "devices", "list", "--json", "--select", "siteId")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if !strings.Contains(out, `"siteId"`) {
		t.Fatal(out)
	}
}

func TestNetworkClientsListTable(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"site-1","internalReference":"default","name":"lab"}]}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/clients", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":2,"totalCount":40,"data":[{"id":"c1","name":"pi-4","ipAddress":"192.168.5.221","macAddress":"dc:a6:32:da:e3:af","type":"WIRED"},{"id":"c2","name":"tasplug-07","ipAddress":"192.168.25.180","macAddress":"aa:bb:cc:dd:ee:ff","type":"WIRELESS"}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "network", "clients", "list", "--plain")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "pi-4") || !strings.Contains(out, "MAC") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "showing 1-2 of 40") || !strings.Contains(out, "--offset 2") {
		t.Fatalf("missing pagination footer: %s", out)
	}
	if strings.Contains(out, `"page"`) {
		t.Fatalf("table path leaked JSON: %s", out)
	}
}

func TestNetworkWifiGetRedactsSecrets(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"site-1","internalReference":"default","name":"lab"}]}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/wifi/broadcasts/wifi-1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"wifi-1","name":"Office","presharedKeys":[{"vlanId":20,"passphrase":"super-secret"}]}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/networks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"net-1","name":"LAN","vlanId":1,"enabled":true}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "network", "wifi", "get", "wifi-1", "--json")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if strings.Contains(out, "super-secret") {
		t.Fatalf("secret leaked: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("expected redaction: %s", out)
	}

	out, _, err = execCLI(t, "network", "wifi", "get", "wifi-1", "--json", "--include-secrets")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if !strings.Contains(out, "super-secret") {
		t.Fatalf("expected secret with flag: %s", out)
	}

	out, _, err = execCLI(t, "network", "networks", "list", "--json")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if !strings.Contains(out, `"LAN"`) {
		t.Fatal(out)
	}
}

func TestRestartBlockedViaCLI(t *testing.T) {
	clearUnifiEnv(t)
	t.Setenv("UNIFI_HOST", "https://example.invalid")
	t.Setenv("UNIFI_API_KEY", "k")
	_, _, err := execCLI(t, "network", "devices", "restart", "dev-1", "--json")
	if exitCode(err) != exitcode.MutationBlocked {
		t.Fatalf("code=%d err=%v", exitCode(err), err)
	}
}

func TestCompleteProfileNames(t *testing.T) {
	clearUnifiEnv(t)
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte("current: dukielska\nprofiles:\n  dukielska:\n    host: https://192.168.5.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rootOpts.configPath = cfg
	t.Cleanup(func() { rootOpts = rootOptions{} })
	names, _ := completeProfileNames(nil, nil, "duk")
	if len(names) != 1 || names[0] != "dukielska" {
		t.Fatalf("%v", names)
	}
}

func TestNetworkGetByNameAndEnableBlocked(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"site-1","internalReference":"default","name":"lab"}]}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/wifi/broadcasts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"wifi-1","name":"Office","enabled":true,"type":"STANDARD"}]}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/wifi/broadcasts/wifi-1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"wifi-1","name":"Office","enabled":true}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "network", "wifi", "get", "Office", "--json")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if !strings.Contains(out, `"wifi-1"`) {
		t.Fatal(out)
	}

	_, _, err = execCLI(t, "network", "wifi", "disable", "Office", "--json")
	if exitCode(err) != exitcode.MutationBlocked {
		t.Fatalf("code=%d err=%v", exitCode(err), err)
	}
}

func TestNetworkCreateFromJSON(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"site-1","internalReference":"default","name":"lab"}]}`))
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/networks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"net-new","name":"IoT","vlanId":30}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "network", "networks", "create", "--json", "--allow-mutations", "--yes",
		"--from-json", `{"name":"IoT","vlanId":30,"management":"UNMANAGED","enabled":true}`)
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if !strings.Contains(out, `"net-new"`) {
		t.Fatal(out)
	}
}

func TestLegacyRoutesList(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":1,"totalCount":1,"data":[{"id":"site-1","internalReference":"default","name":"lab"}]}`))
	})
	mux.HandleFunc("/proxy/network/api/s/default/rest/routing", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"_id":"r1","name":"GLUCHOW-217","static-route_network":"192.168.6.0/24","static-route_nexthop":"192.168.5.221","enabled":true}]}`))
	})
	mux.HandleFunc("/proxy/network/api/s/default/rest/portforward", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"_id":"pf1","name":"NAS HTTPS","enabled":true,"dst_port":"443","fwd":"10.5.67.10","proto":"tcp"}]}`))
	})
	mux.HandleFunc("/proxy/network/api/s/default/stat/device", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"_id":"sw1","name":"MINI RACK","mac":"aa:bb:cc:dd:ee:ff","port_table":[{"port_idx":2,"name":"PDU","up":true,"speed":1000,"poe_mode":"auto","portconf_id":"prof1"}]}]}`))
	})
	mux.HandleFunc("/proxy/network/v2/api/site/default/firewall-policies", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"_id":"pol1","name":"Allow PDM to hive-01","index":10000,"action":"ALLOW","enabled":true,"hits":12,"predefined":false}]`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "network", "routes", "list", "--json")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if !strings.Contains(out, `"legacy-controller"`) || !strings.Contains(out, `"GLUCHOW-217"`) {
		t.Fatal(out)
	}

	out, _, err = execCLI(t, "network", "port-forwards", "list", "--json")
	if err != nil {
		t.Fatalf("port-forwards %v %s", err, out)
	}
	if !strings.Contains(out, `"NAS HTTPS"`) {
		t.Fatal(out)
	}

	out, _, err = execCLI(t, "network", "ports", "list", "--json")
	if err != nil {
		t.Fatalf("ports %v %s", err, out)
	}
	if !strings.Contains(out, `"MINI RACK"`) || !strings.Contains(out, `"PDU"`) {
		t.Fatal(out)
	}

	out, _, err = execCLI(t, "network", "firewall", "policies", "list", "--json")
	if err != nil {
		t.Fatalf("policies %v %s", err, out)
	}
	if !strings.Contains(out, `"Allow PDM to hive-01"`) || !strings.Contains(out, `"pol1"`) {
		t.Fatal(out)
	}
}

func TestProtectNVRAndStreamRedact(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/protect/integration/v1/nvrs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"nvr-1","modelKey":"nvr","name":"lab-nvr"}`))
	})
	mux.HandleFunc("/proxy/protect/integration/v1/cameras", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"cam-1","modelKey":"camera","state":"CONNECTED","name":"vestibule"}]`))
	})
	mux.HandleFunc("/proxy/protect/integration/v1/cameras/cam-1/rtsps-stream", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"high":"rtsps://192.168.5.1:7441/secretToken?enableSrtp"}`))
	})
	mux.HandleFunc("/proxy/protect/integration/v1/liveviews", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"lv-1","name":"Default","modelKey":"liveview"}]`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "protect", "nvr", "--json")
	if err != nil {
		t.Fatalf("nvr %v %s", err, out)
	}
	if !strings.Contains(out, `"lab-nvr"`) {
		t.Fatal(out)
	}
	out, _, err = execCLI(t, "protect", "cameras", "stream", "vestibule", "--json")
	if err != nil {
		t.Fatalf("stream %v %s", err, out)
	}
	if strings.Contains(out, "secretToken") || !strings.Contains(out, "[redacted]") {
		t.Fatalf("expected redacted stream: %s", out)
	}
	out, _, err = execCLI(t, "protect", "liveviews", "list", "--json")
	if err != nil {
		t.Fatalf("liveviews %v %s", err, out)
	}
	if !strings.Contains(out, `"Default"`) {
		t.Fatal(out)
	}
}

func TestProtectCameraSetAndAccessVisitors(t *testing.T) {
	clearUnifiEnv(t)
	t.Cleanup(func() { rootOpts = rootOptions{} })
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/protect/integration/v1/cameras", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"cam-1","modelKey":"camera","state":"CONNECTED","name":"vestibule"}]`))
	})
	mux.HandleFunc("/proxy/protect/integration/v1/cameras/cam-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			_, _ = w.Write([]byte(`{"id":"cam-1","hdrType":"off"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"cam-1","name":"vestibule","state":"CONNECTED","featureFlags":{"hasHdr":true}}`))
	})
	mux.HandleFunc("/proxy/access/api/v2/visitors", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"v1","name":"Guest","status":"ACTIVE"}]`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UNIFI_HOST", srv.URL)
	t.Setenv("UNIFI_API_KEY", "k")

	out, _, err := execCLI(t, "protect", "cameras", "get", "vestibule", "--json")
	if err != nil {
		t.Fatalf("get %v %s", err, out)
	}
	if !strings.Contains(out, `"hasHdr"`) {
		t.Fatal(out)
	}
	out, _, err = execCLI(t, "protect", "cameras", "set", "vestibule", "--hdr", "off", "--json", "--allow-mutations", "--yes")
	if err != nil {
		t.Fatalf("set %v %s", err, out)
	}
	if !strings.Contains(out, `"off"`) {
		t.Fatal(out)
	}
	out, _, err = execCLI(t, "access", "visitors", "list", "--json")
	if err != nil {
		t.Fatalf("visitors %v %s", err, out)
	}
	if !strings.Contains(out, `"Guest"`) {
		t.Fatal(out)
	}
}
