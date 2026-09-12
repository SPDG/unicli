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

func redactTraffic(s string) string {
	out := strings.NewReplacer("leska", "site", "dukielska", "profile").Replace(s)
	for {
		i := strings.Index(out, `"mac": "`)
		if i < 0 {
			break
		}
		j := strings.Index(out[i+8:], `"`)
		if j < 0 {
			break
		}
		out = out[:i+8] + "aa:00:00:00:00:00" + out[i+8+j:]
	}
	return out
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
			t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, redactTraffic(string(out)))
		}
		return string(out)
	}

	doctor := run("doctor", "--json")
	if !strings.Contains(doctor, `"ok": true`) {
		t.Fatalf("doctor: %s", doctor)
	}
	status := run("console", "status", "--json")
	if !strings.Contains(status, `"system"`) || !strings.Contains(status, `"apps"`) {
		t.Fatalf("console status: %s", status)
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
	if !strings.Contains(pols, "firewall-policies") && !strings.Contains(pols, "siteId") {
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
	nvr := run("protect", "nvr", "--json")
	if !strings.Contains(nvr, `"modelKey"`) {
		t.Fatalf("nvr: %s", nvr)
	}
	sys := run("network", "sysinfo", "--json")
	if !strings.Contains(sys, "legacy-controller") && !strings.Contains(sys, "version") {
		t.Fatalf("sysinfo: %s", sys)
	}
	wan := run("network", "traffic", "wan", "--json", "--timezone", "Europe/Warsaw")
	if !strings.Contains(wan, `"schema": "unicli.network.traffic.wan/v1"`) || !strings.Contains(wan, `"scope": "wan"`) {
		t.Fatalf("traffic wan: %s", redactTraffic(wan))
	}
	if strings.Contains(wan, `"totals": 0`) {
		t.Fatalf("missing WAN data must not be a bare zero: %s", redactTraffic(wan))
	}
	clients := run("network", "traffic", "clients", "--json", "--timezone", "Europe/Warsaw", "--sort", "total", "--top", "5")
	if !strings.Contains(clients, `"schema": "unicli.network.traffic.clients/v1"`) || !strings.Contains(clients, `"scope": "client-all-traffic"`) {
		t.Fatalf("traffic clients: %s", redactTraffic(clients))
	}
	if strings.Contains(clients, `"Internet"`) {
		t.Fatalf("must not label client totals as Internet: %s", redactTraffic(clients))
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
