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
