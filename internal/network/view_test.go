package network

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeMAC(t *testing.T) {
	if got := NormalizeMAC("DC-A6-32-DA-E3-AF"); got != "dc:a6:32:da:e3:af" {
		t.Fatalf("%s", got)
	}
	if !LooksLikeMAC("dc:a6:32:da:e3:af") || LooksLikeMAC("pi-4") {
		t.Fatal("LooksLikeMAC")
	}
}

func TestMergeClientViewAndPort(t *testing.T) {
	intg := &Client{ID: "c1", Name: "pi", Type: "WIRED", MacAddress: "dc:a6:32:da:e3:af", IPAddress: "192.168.5.221", ConnectedAt: "2026-01-01T00:00:00Z"}
	active := map[string]any{
		"mac": "dc:a6:32:da:e3:af", "ip": "192.168.5.221", "vlan": 20.0, "network_name": "IoT",
		"authorized": true, "blocked": false, "status": "online", "sw_port": 7.0,
		"uplink_mac": "aa:bb:cc:dd:ee:ff", "last_uplink_name": "SW1", "wired_rate_mbps": 1000.0,
		"last_seen": 1.7e9, "uptime": 60.0, "is_wired": true,
	}
	devs := []map[string]any{{
		"_id": "sw1", "name": "SW1", "mac": "aa:bb:cc:dd:ee:ff",
		"port_table": []any{map[string]any{
			"port_idx": 7.0, "name": "Pi", "up": true, "speed": 1000.0, "rx_errors": 1.0, "tx_dropped": 2.0,
			"link_down_count": 3.0, "lag_member": true, "lag_idx": 1.0, "aggregate_members": []any{7.0, 8.0},
		}},
	}}
	v := MergeClientView(intg, active, nil, devs)
	if v.VLAN == nil || *v.VLAN != 20 || v.Network != "IoT" || v.Uplink == nil || v.Uplink.Port == nil || *v.Uplink.Port != 7 {
		t.Fatalf("%+v", v)
	}
	if v.Uplink.PortName != "Pi" || v.Uplink.LAG == nil || !v.Uplink.LAG.Member {
		t.Fatalf("uplink %+v", v.Uplink)
	}
	if v.Uplink.RXErrors == nil || *v.Uplink.RXErrors != 1 {
		t.Fatalf("errors %+v", v.Uplink)
	}
}

func TestTracePath(t *testing.T) {
	topo := Topology{
		Vertices: []TopologyVertex{
			{Type: "CLIENT", MAC: "aa:aa:aa:aa:aa:01", Name: "a"},
			{Type: "DEVICE", MAC: "aa:aa:aa:aa:aa:s1", Name: "sw1", Model: "USMINI"},
			{Type: "DEVICE", MAC: "aa:aa:aa:aa:aa:gw", Name: "udm", Model: "UDMPRO"},
			{Type: "DEVICE", MAC: "aa:aa:aa:aa:aa:s2", Name: "sw2", Model: "USMINI"},
			{Type: "CLIENT", MAC: "aa:aa:aa:aa:aa:02", Name: "b"},
		},
		Edges: []TopologyEdge{
			{Type: "WIRED", DownlinkMAC: "aa:aa:aa:aa:aa:01", UplinkMAC: "aa:aa:aa:aa:aa:s1", UplinkPortNumber: intPtr(3), RateMbps: intPtr(1000)},
			{Type: "WIRED", DownlinkMAC: "aa:aa:aa:aa:aa:s1", UplinkMAC: "aa:aa:aa:aa:aa:gw", UplinkPortNumber: intPtr(9), RateMbps: intPtr(10000)},
			{Type: "WIRED", DownlinkMAC: "aa:aa:aa:aa:aa:s2", UplinkMAC: "aa:aa:aa:aa:aa:gw", UplinkPortNumber: intPtr(10), RateMbps: intPtr(10000)},
			{Type: "WIRED", DownlinkMAC: "aa:aa:aa:aa:aa:02", UplinkMAC: "aa:aa:aa:aa:aa:s2", UplinkPortNumber: intPtr(1), RateMbps: intPtr(1000)},
		},
	}
	tr := TracePath(topo, []map[string]any{
		{"name": "sw1", "mac": "aa:aa:aa:aa:aa:s1", "port_table": []any{map[string]any{"port_idx": 3.0, "name": "A", "speed": 1000.0}}},
		{"name": "udm", "mac": "aa:aa:aa:aa:aa:gw", "lan_ip": "192.168.5.1", "port_table": []any{map[string]any{"port_idx": 9.0, "name": "SFP", "speed": 10000.0}}},
		{"name": "sw2", "mac": "aa:aa:aa:aa:aa:s2", "port_table": []any{map[string]any{"port_idx": 1.0, "name": "B", "speed": 1000.0}}},
	}, "AA-AA-AA-AA-AA-01", "aa:aa:aa:aa:aa:02")
	if !tr.Found || !tr.Complete || len(tr.Hops) != 5 {
		t.Fatalf("%+v", tr)
	}
	if tr.Hops[0].Name != "a" || tr.Hops[4].Name != "b" || tr.Hops[2].Model != "UDMPRO" {
		t.Fatalf("%+v", tr.Hops)
	}
	if tr.Hops[4].Uplink == nil || tr.Hops[4].Uplink.Device != "sw2" || tr.Hops[4].Uplink.Port == nil || *tr.Hops[4].Uplink.Port != 1 {
		t.Fatalf("last hop uplink %+v", tr.Hops[4].Uplink)
	}
	chain := UplinkChain(topo, nil, "aa:aa:aa:aa:aa:01")
	if len(chain) != 3 || chain[2].Name != "udm" {
		t.Fatalf("%+v", chain)
	}
}

func TestTracePathVirtualBehindHost(t *testing.T) {
	topo := Topology{
		Vertices: []TopologyVertex{
			{Type: "CLIENT", MAC: "aa:aa:aa:aa:aa:01", Name: "vm-a"},
			{Type: "CLIENT", MAC: "aa:aa:aa:aa:aa:02", Name: "vm-b"},
			{Type: "DEVICE", MAC: "aa:aa:aa:aa:aa:s1", Name: "sw1"},
			{Type: "DEVICE", MAC: "aa:aa:aa:aa:aa:gw", Name: "udm"},
		},
		Edges: []TopologyEdge{
			{Type: "WIRED", DownlinkMAC: "aa:aa:aa:aa:aa:01", UplinkMAC: "aa:aa:aa:aa:aa:s1", UplinkPortNumber: intPtr(1), RateMbps: intPtr(10000)},
			{Type: "WIRED", DownlinkMAC: "aa:aa:aa:aa:aa:02", UplinkMAC: "aa:aa:aa:aa:aa:s1", UplinkPortNumber: intPtr(1), RateMbps: intPtr(10000)},
			{Type: "WIRED", DownlinkMAC: "aa:aa:aa:aa:aa:s1", UplinkMAC: "aa:aa:aa:aa:aa:gw", UplinkPortNumber: intPtr(9), RateMbps: intPtr(10000)},
		},
	}
	devs := []map[string]any{{"name": "sw1", "mac": "aa:aa:aa:aa:aa:s1", "port_table": []any{map[string]any{"port_idx": 1.0, "name": "LAG1", "speed": 10000.0}}}}
	tr := TracePath(topo, devs, "aa:aa:aa:aa:aa:01", "aa:aa:aa:aa:aa:gw")
	if !tr.Found || tr.Complete {
		t.Fatalf("want incomplete path %+v", tr)
	}
	if tr.Hops[0].Attachment != AttachmentVirtual || tr.Hops[0].Uplink == nil || tr.Hops[0].Uplink.Device != "sw1" {
		t.Fatalf("vm hop %+v", tr.Hops[0])
	}
	if tr.Hops[0].SharedOnPort == nil || *tr.Hops[0].SharedOnPort != 1 {
		t.Fatalf("sharedOnPort=%v", tr.Hops[0].SharedOnPort)
	}
	if tr.Hops[0].SharedCount == nil || *tr.Hops[0].SharedCount != 2 {
		t.Fatalf("sharedCount=%v", tr.Hops[0].SharedCount)
	}
}

func TestDeviceIPsIncludesLAN(t *testing.T) {
	ips := DeviceIPs(map[string]any{
		"ip": "185.1.2.3", "lan_ip": "192.168.5.1",
		"network_table": []any{map[string]any{"ip": "192.168.20.1"}},
	})
	joined := strings.Join(ips, ",")
	if !strings.Contains(joined, "192.168.5.1") || !strings.Contains(joined, "185.1.2.3") || !strings.Contains(joined, "192.168.20.1") {
		t.Fatalf("%v", ips)
	}
}

func TestAnnotateHealthVPNUnknown(t *testing.T) {
	rows := AnnotateHealth([]map[string]any{
		{"subsystem": "lan", "status": "ok", "num_user": 10.0},
		{"subsystem": "vpn", "status": "unknown"},
	}, 0, 0)
	if rows[1].Note == "" || rows[0].Note != "" {
		t.Fatalf("%+v", rows)
	}
}

func TestFindPortsByClientMAC(t *testing.T) {
	devs := []map[string]any{{
		"name": "SW1", "mac": "aa:bb:cc:dd:ee:ff", "_id": "sw1",
		"port_table": []any{
			map[string]any{"port_idx": 1.0, "name": "A", "up": true, "speed": 1000.0},
			map[string]any{"port_idx": 2.0, "name": "B", "up": true, "speed": 100.0},
		},
	}}
	clients := []map[string]any{{
		"mac": "11:22:33:44:55:66", "ip": "10.0.0.8", "display_name": "cam",
		"sw_port": 2.0, "uplink_mac": "aa:bb:cc:dd:ee:ff",
	}}
	found := FindPorts(devs, clients, "11-22-33-44-55-66", "", "")
	if len(found) != 1 || found[0].Port != 2 || found[0].Name != "B" {
		t.Fatalf("%+v", found)
	}
}

func TestTopologyFetchAndLookup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/v2/api/site/default/topology", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"vertices": []map[string]any{{"type": "CLIENT", "mac": "11:22:33:44:55:66", "name": "cam"}},
			"edges": []map[string]any{{"type": "WIRED", "downlinkMac": "11:22:33:44:55:66", "uplinkMac": "aa:bb:cc:dd:ee:ff", "uplinkPortNumber": 2, "rateMbps": 1000}},
		})
	})
	mux.HandleFunc("/proxy/network/v2/api/site/default/clients/active", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"mac": "11:22:33:44:55:66", "ip": "10.0.0.8", "display_name": "cam", "vlan": 20, "sw_port": 2, "uplink_mac": "aa:bb:cc:dd:ee:ff",
		}})
	})
	mux.HandleFunc("/proxy/network/api/s/default/stat/sta", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]string{"rc": "ok"}, "data": []any{}})
	})
	mux.HandleFunc("/proxy/network/api/s/default/stat/device", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]string{"rc": "ok"}, "data": []map[string]any{{
			"name": "SW1", "mac": "aa:bb:cc:dd:ee:ff", "port_table": []map[string]any{{"port_idx": 2, "name": "Cam", "up": true, "speed": 1000}},
		}}})
	})
	api := testAPI(t, mux)
	topo, err := api.Topology(context.Background(), "default")
	if err != nil || len(topo.Edges) != 1 {
		t.Fatalf("%v %+v", err, topo)
	}
	view, err := api.LookupClientView(context.Background(), "", "default", "10.0.0.8")
	if err != nil || view.VLAN == nil || *view.VLAN != 20 || view.Uplink == nil || view.Uplink.PortName != "Cam" {
		t.Fatalf("%v %+v", err, view)
	}
}

func TestResolveDeviceByLANIP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/v2/api/site/default/clients/active", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{})
	})
	mux.HandleFunc("/proxy/network/api/s/default/stat/sta", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]string{"rc": "ok"}, "data": []any{}})
	})
	mux.HandleFunc("/proxy/network/api/s/default/stat/device", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]string{"rc": "ok"}, "data": []map[string]any{{
			"name": "rymowanki-dukielska", "mac": "74:ac:b9:d9:55:ed", "model": "UDMPRO",
			"ip": "185.1.2.3", "lan_ip": "192.168.5.1",
		}}})
	})
	api := testAPI(t, mux)
	view, err := api.ResolveMAC(context.Background(), "", "default", "192.168.5.1")
	if err != nil || view.Type != "DEVICE" || view.Name != "rymowanki-dukielska" || view.MacAddress != "74:ac:b9:d9:55:ed" {
		t.Fatalf("%v %+v", err, view)
	}
}

func intPtr(n int) *int { return &n }
