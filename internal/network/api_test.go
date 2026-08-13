package network

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestNetworksWifiFirewallAndACL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/networks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[Network]{
			Count: 1, TotalCount: 1,
			Data: []Network{{ID: "net-1", Name: "LAN", VlanID: 1, Enabled: true, Management: "GATEWAY"}},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/networks/net-1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "net-1", "name": "LAN", "vlanId": 1})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/wifi/broadcasts", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[WifiBroadcast]{
			Count: 1, TotalCount: 1,
			Data: []WifiBroadcast{{ID: "wifi-1", Name: "Office", Enabled: true, Type: "STANDARD"}},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/wifi/broadcasts/wifi-1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":            "wifi-1",
			"presharedKeys": []map[string]any{{"passphrase": "secret-psk"}},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/firewall/zones", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[FirewallZone]{
			Count: 1, TotalCount: 1,
			Data: []FirewallZone{{ID: "zone-1", Name: "Internal"}},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/firewall/zones/zone-1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "zone-1", "name": "Internal"})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/firewall/policies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[FirewallPolicy]{
			Count: 1, TotalCount: 1,
			Data: []FirewallPolicy{{Name: "Allow LAN", Enabled: true, Index: 2000}},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/firewall/policies/pol-1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "pol-1", "name": "Allow LAN"})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/acl-rules", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[ACLRule]{Count: 0, TotalCount: 0, Data: []ACLRule{}})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/acl-rules/acl-1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "acl-1", "name": "block"})
	})

	api := testAPI(t, mux)
	ctx := context.Background()
	nets, err := api.Networks(ctx, "site-1", 0, 25)
	if err != nil || nets.Data[0].VlanID != 1 {
		t.Fatalf("%v %+v", err, nets)
	}
	net, err := api.Network(ctx, "site-1", "net-1")
	if err != nil || net["name"] != "LAN" {
		t.Fatalf("%v %+v", err, net)
	}
	wifi, err := api.WifiBroadcasts(ctx, "site-1", 0, 25)
	if err != nil || wifi.Data[0].Name != "Office" {
		t.Fatalf("%v %+v", err, wifi)
	}
	detail, err := api.WifiBroadcast(ctx, "site-1", "wifi-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := detail["presharedKeys"]; !ok {
		t.Fatalf("%v", detail)
	}
	zones, err := api.FirewallZones(ctx, "site-1", 0, 25)
	if err != nil || zones.Data[0].Name != "Internal" {
		t.Fatalf("%v %+v", err, zones)
	}
	if _, err := api.FirewallZone(ctx, "site-1", "zone-1"); err != nil {
		t.Fatal(err)
	}
	pols, err := api.FirewallPolicies(ctx, "site-1", 0, 25)
	if err != nil || pols.Data[0].ID != "" || pols.Data[0].Name != "Allow LAN" {
		t.Fatalf("%v %+v", err, pols)
	}
	if _, err := api.FirewallPolicy(ctx, "site-1", "pol-1"); err != nil {
		t.Fatal(err)
	}
	acls, err := api.ACLRules(ctx, "site-1", 0, 25)
	if err != nil || acls.TotalCount != 0 {
		t.Fatalf("%v %+v", err, acls)
	}
	if _, err := api.ACLRule(ctx, "site-1", "acl-1"); err != nil {
		t.Fatal(err)
	}
}

func TestCollectAll(t *testing.T) {
	calls := 0
	fetch := func(offset, limit int) (*Page[int], error) {
		calls++
		if offset == 0 {
			return &Page[int]{TotalCount: 3, Count: 2, Data: []int{1, 2}}, nil
		}
		return &Page[int]{TotalCount: 3, Count: 1, Data: []int{3}}, nil
	}
	page, err := Collect(fetch, 0, 25, true)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || page.TotalCount != 3 || len(page.Data) != 3 {
		t.Fatalf("calls=%d page=%+v", calls, page)
	}
	one, err := Collect(fetch, 0, 25, false)
	if err != nil || one.TotalCount != 3 {
		t.Fatal(err)
	}
}

func TestDropReadOnly(t *testing.T) {
	in := map[string]any{"id": "x", "metadata": map[string]any{"a": 1}, "default": true, "name": "LAN", "enabled": true}
	out := DropReadOnly(in)
	if _, ok := out["id"]; ok {
		t.Fatal(out)
	}
	if out["name"] != "LAN" || in["id"] != "x" {
		t.Fatalf("mutated original or dropped name: %v %v", out, in)
	}
}

func TestNetworkCreateUpdateDelete(t *testing.T) {
	var putBody, postBody []byte
	var deleted bool
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/networks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "net-new", "name": "IoT"})
			return
		}
		_ = json.NewEncoder(w).Encode(Page[Network]{
			Count: 1, TotalCount: 1, Data: []Network{{ID: "net-1", Name: "LAN", VlanID: 1}},
		})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/networks/net-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			putBody, _ = io.ReadAll(r.Body)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "net-1", "enabled": false})
		case http.MethodDelete:
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "net-1", "name": "LAN", "enabled": true, "metadata": map[string]any{"x": 1}})
		}
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/firewall/policies/pol-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "pol-1", "loggingEnabled": true})
	})
	mux.HandleFunc("/proxy/network/integration/v1/sites/site-1/dns/policies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[map[string]any]{Count: 0, TotalCount: 0, Data: []map[string]any{}})
	})

	api := testAPI(t, mux)
	ctx := context.Background()
	created, err := api.CreateNetwork(ctx, "site-1", map[string]any{"name": "IoT", "vlanId": 30, "management": "UNMANAGED", "enabled": true})
	if err != nil || created["id"] != "net-new" {
		t.Fatalf("%v %v", err, created)
	}
	if !strings.Contains(string(postBody), `"vlanId":30`) {
		t.Fatalf("post=%s", postBody)
	}
	cur, err := api.Network(ctx, "site-1", "net-1")
	if err != nil {
		t.Fatal(err)
	}
	body := DropReadOnly(cur)
	body["enabled"] = false
	if _, err := api.UpdateNetwork(ctx, "site-1", "net-1", body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(putBody), `"metadata"`) || strings.Contains(string(putBody), `"id"`) {
		t.Fatalf("put leaked readonly: %s", putBody)
	}
	if err := api.DeleteNetwork(ctx, "site-1", "net-1", true); err != nil || !deleted {
		t.Fatalf("delete %v deleted=%v", err, deleted)
	}
	if _, err := api.PatchFirewallPolicy(ctx, "site-1", "pol-1", map[string]any{"loggingEnabled": true}); err != nil {
		t.Fatal(err)
	}
	dns, err := api.DNSPolicies(ctx, "site-1", 0, 25)
	if err != nil || dns.TotalCount != 0 {
		t.Fatalf("%v %+v", err, dns)
	}
}

func TestLegacyRoutingPortConfAndGroups(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page[Site]{
			Count: 1, TotalCount: 1,
			Data: []Site{{ID: "uuid-1", InternalRef: "default", Name: "leska"}},
		})
	})
	mux.HandleFunc("/proxy/network/api/s/default/rest/routing", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]string{"rc": "ok"},
			"data": []map[string]any{{
				"_id": "r1", "name": "GLUCHOW-217", "static-route_network": "192.168.6.0/24",
				"static-route_nexthop": "192.168.5.221", "enabled": true,
			}},
		})
	})
	mux.HandleFunc("/proxy/network/api/s/default/rest/portconf", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]string{"rc": "ok"},
			"data": []map[string]any{{"_id": "p1", "name": "PROXMOX MGMT", "poe_mode": "off", "forward": "native"}},
		})
	})
	mux.HandleFunc("/proxy/network/api/s/default/rest/firewallgroup", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]string{"rc": "ok"},
			"data": []map[string]any{{"_id": "g1", "name": "K8S-RFC1918", "group_type": "address-group", "group_members": []string{"10.0.0.0/8"}}},
		})
	})
	mux.HandleFunc("/proxy/network/v2/api/site/default/trafficroutes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{})
	})

	api := testAPI(t, mux)
	ctx := context.Background()
	slug, err := api.LegacySiteSlug(ctx, "uuid-1")
	if err != nil || slug != "default" {
		t.Fatalf("%v %q", err, slug)
	}
	routes, err := api.LegacyList(ctx, "default", "routing")
	if err != nil || routes[0]["name"] != "GLUCHOW-217" {
		t.Fatalf("%v %+v", err, routes)
	}
	ports, err := api.LegacyList(ctx, "default", "portconf")
	if err != nil || ports[0]["name"] != "PROXMOX MGMT" {
		t.Fatalf("%v %+v", err, ports)
	}
	groups, err := api.LegacyList(ctx, "default", "firewallgroup")
	if err != nil || groups[0]["name"] != "K8S-RFC1918" {
		t.Fatalf("%v %+v", err, groups)
	}
	tr, err := api.TrafficRoutes(ctx, "default")
	if err != nil || len(tr) != 0 {
		t.Fatalf("%v %+v", err, tr)
	}
}

func TestStatDevicesAndV2Firewall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/api/s/default/stat/device", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]string{"rc": "ok"},
			"data": []map[string]any{{
				"_id": "sw1", "name": "MINI RACK", "mac": "aa:bb:cc:dd:ee:ff",
				"port_table": []map[string]any{{"port_idx": 1, "name": "UPLINK", "up": true, "speed": 1000}},
			}},
		})
	})
	mux.HandleFunc("/proxy/network/v2/api/site/default/firewall-policies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"_id": "pol1", "name": "Allow PDM to hive-01", "index": 10000, "action": "ALLOW", "enabled": true, "hits": 12, "predefined": false,
		}})
	})
	api := testAPI(t, mux)
	ctx := context.Background()
	devs, err := api.StatDevices(ctx, "default")
	if err != nil || devs[0]["name"] != "MINI RACK" {
		t.Fatalf("%v %+v", err, devs)
	}
	pols, err := api.V2FirewallPolicies(ctx, "default")
	if err != nil || LegacyID(pols[0]) != "pol1" {
		t.Fatalf("%v %+v", err, pols)
	}
}

func TestPageQueryDefaults(t *testing.T) {
	q := pageQuery(-1, 0)
	if q.Get("offset") != "0" || q.Get("limit") != "25" {
		t.Fatalf("%v", q)
	}
}
