//go:build live

package live

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func unicliBin(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	bin := filepath.Join(t.TempDir(), "unicli")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/unicli")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build unicli: %v\n%s", err, out)
	}
	return bin
}

func TestLiveDoctorAndLists(t *testing.T) {
	if os.Getenv("UNIFI_HOST") == "" || os.Getenv("UNIFI_API_KEY") == "" {
		t.Skip("set UNIFI_HOST and UNIFI_API_KEY to run live tests")
	}
	bin := unicliBin(t)
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}

	doctor := run("doctor", "--json")
	if !strings.Contains(doctor, `"ok": true`) {
		t.Fatalf("doctor: %s", doctor)
	}
	info := run("network", "info", "--json")
	if !strings.Contains(info, "applicationVersion") {
		t.Fatalf("info: %s", info)
	}
	devs := run("network", "devices", "list", "--json", "--limit", "1")
	if !strings.Contains(devs, "siteId") {
		t.Fatalf("devices: %s", devs)
	}
	nets := run("network", "networks", "list", "--json", "--limit", "1")
	if !strings.Contains(nets, "siteId") {
		t.Fatalf("networks: %s", nets)
	}
	wifi := run("network", "wifi", "list", "--json", "--limit", "1")
	if !strings.Contains(wifi, "siteId") {
		t.Fatalf("wifi: %s", wifi)
	}
	if strings.Contains(strings.ToLower(wifi), "passphrase") {
		t.Fatalf("wifi list should not include passphrases: %s", wifi)
	}
	zones := run("network", "firewall", "zones", "list", "--json", "--limit", "5")
	if !strings.Contains(zones, "siteId") {
		t.Fatalf("zones: %s", zones)
	}
	pols := run("network", "firewall", "policies", "list", "--json", "--limit", "1")
	if !strings.Contains(pols, "siteId") {
		t.Fatalf("policies: %s", pols)
	}
	acls := run("network", "acl", "list", "--json", "--limit", "1")
	if !strings.Contains(acls, "siteId") {
		t.Fatalf("acl: %s", acls)
	}
	dns := run("network", "dns", "list", "--json", "--limit", "1")
	if !strings.Contains(dns, "siteId") {
		t.Fatalf("dns: %s", dns)
	}
	wans := run("network", "wans", "--json", "--limit", "1")
	if !strings.Contains(wans, "siteId") {
		t.Fatalf("wans: %s", wans)
	}
	routes := run("network", "routes", "list", "--json")
	if !strings.Contains(routes, "legacy-controller") {
		t.Fatalf("routes: %s", routes)
	}
	fwd := run("network", "port-forwards", "list", "--json")
	if !strings.Contains(fwd, "legacy-controller") {
		t.Fatalf("port-forwards: %s", fwd)
	}
	protect := run("protect", "info", "--json")
	if !strings.Contains(protect, "applicationVersion") {
		t.Fatalf("protect: %s", protect)
	}

	cmd := exec.Command(bin, "access", "info", "--json")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Logf("access present: %s", out)
		return
	}
	if cmd.ProcessState.ExitCode() != 11 {
		t.Fatalf("access info: want exit 11 when missing, got %d\n%s", cmd.ProcessState.ExitCode(), out)
	}
}
