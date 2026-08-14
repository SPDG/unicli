package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func (a *API) Topology(ctx context.Context, siteSlug string) (Topology, error) {
	var raw json.RawMessage
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/topology"
	if err := a.c.GetPathJSON(ctx, path, nil, &raw); err != nil {
		return Topology{}, err
	}
	var topo struct {
		Vertices         []map[string]any `json:"vertices"`
		Edges            []map[string]any `json:"edges"`
		HasUnknownSwitch bool             `json:"has_unknown_switch"`
	}
	if err := json.Unmarshal(raw, &topo); err != nil {
		return Topology{}, fmt.Errorf("decode topology: %w", err)
	}
	out := Topology{HasUnknownSwitch: topo.HasUnknownSwitch}
	for _, v := range topo.Vertices {
		out.Vertices = append(out.Vertices, TopologyVertex{
			Type:  str(v["type"]),
			MAC:   NormalizeMAC(str(v["mac"])),
			Name:  str(v["name"]),
			Model: str(v["model"]),
			State: v["state"],
		})
	}
	for _, e := range topo.Edges {
		out.Edges = append(out.Edges, TopologyEdge{
			Type:             str(e["type"]),
			UplinkMAC:        NormalizeMAC(str(e["uplinkMac"])),
			DownlinkMAC:      NormalizeMAC(str(e["downlinkMac"])),
			UplinkPortNumber: asIntPtr(e["uplinkPortNumber"]),
			RateMbps:         asIntPtr(e["rateMbps"]),
			NetworkID:        str(e["networkId"]),
			ESSID:            str(e["essid"]),
			RadioBand:        str(e["radioBand"]),
			Channel:          asIntPtr(e["channel"]),
		})
	}
	return out, nil
}

func (a *API) ActiveClients(ctx context.Context, siteSlug string) ([]map[string]any, error) {
	var raw json.RawMessage
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/clients/active"
	if err := a.c.GetPathJSON(ctx, path, nil, &raw); err != nil {
		return nil, err
	}
	return decodeLegacyList(raw)
}

func (a *API) StatSta(ctx context.Context, siteSlug string) ([]map[string]any, error) {
	return a.legacyStat(ctx, siteSlug, "sta")
}

func (a *API) LookupClientView(ctx context.Context, siteID, siteSlug, arg string) (ClientView, error) {
	arg = strings.TrimSpace(arg)
	active, err := a.ActiveClients(ctx, siteSlug)
	if err != nil {
		return ClientView{}, err
	}
	sta, _ := a.StatSta(ctx, siteSlug)
	devs, _ := a.StatDevices(ctx, siteSlug)

	var intg *Client
	if siteID != "" && looksLikeClientID(arg) {
		if c, err := a.Client(ctx, siteID, arg); err == nil {
			intg = c
		}
	}
	if intg == nil && siteID != "" {
		if page, err := a.Clients(ctx, siteID, 0, 200); err == nil {
			for i := range page.Data {
				c := page.Data[i]
				if strings.EqualFold(c.ID, arg) || strings.EqualFold(c.Name, arg) || strings.EqualFold(c.IPAddress, arg) || NormalizeMAC(c.MacAddress) == NormalizeMAC(arg) {
					cp := c
					intg = &cp
					break
				}
			}
		}
	}

	var exact, partial []map[string]any
	for _, m := range active {
		view := MergeClientView(nil, m, nil, nil)
		if exactClientArg(arg, view, m) {
			exact = append(exact, m)
			continue
		}
		if strings.Contains(strings.ToLower(view.Name), strings.ToLower(arg)) {
			partial = append(partial, m)
		}
	}
	var activeHit map[string]any
	if intg == nil {
		switch {
		case len(exact) == 1:
			activeHit = exact[0]
		case len(exact) > 1:
			return ClientView{}, fmt.Errorf("ambiguous client %q (%d matches)", arg, len(exact))
		case len(partial) == 1:
			activeHit = partial[0]
		case len(partial) > 1:
			return ClientView{}, fmt.Errorf("ambiguous client %q (%d matches)", arg, len(partial))
		}
	} else {
		activeHit = findClientMap(active, arg, intg)
	}
	staHit := findClientMap(sta, arg, intg)
	if intg == nil && activeHit == nil && staHit == nil {
		return ClientView{}, fmt.Errorf("client %q not found", arg)
	}
	return MergeClientView(intg, activeHit, staHit, devs), nil
}

func exactClientArg(arg string, v ClientView, raw map[string]any) bool {
	if strings.EqualFold(v.ID, arg) || strings.EqualFold(v.Name, arg) || strings.EqualFold(v.IPAddress, arg) {
		return true
	}
	if LooksLikeMAC(arg) && NormalizeMAC(arg) == v.MacAddress {
		return true
	}
	if raw != nil {
		for _, k := range []string{"id", "user_id", "mac", "ip", "name", "display_name", "hostname"} {
			if strings.EqualFold(str(raw[k]), arg) {
				return true
			}
			if k == "mac" && LooksLikeMAC(arg) && NormalizeMAC(arg) == NormalizeMAC(str(raw[k])) {
				return true
			}
		}
	}
	return false
}

func (a *API) ResolveMAC(ctx context.Context, siteID, siteSlug, arg string) (ClientView, error) {
	arg = strings.TrimSpace(arg)
	devs, _ := a.StatDevices(ctx, siteSlug)
	if LooksLikeIP(arg) {
		if view, ok := matchDevice(devs, arg); ok {
			return view, nil
		}
	}
	view, err := a.LookupClientView(ctx, siteID, siteSlug, arg)
	if err == nil {
		return view, nil
	}
	if view, ok := matchDevice(devs, arg); ok {
		return view, nil
	}
	var partial []ClientView
	for _, d := range devs {
		name := str(d["name"])
		if name != "" && strings.Contains(strings.ToLower(name), strings.ToLower(arg)) {
			partial = append(partial, deviceView(d, ""))
		}
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	if len(partial) > 1 {
		return ClientView{}, fmt.Errorf("ambiguous device %q (%d matches)", arg, len(partial))
	}
	if LooksLikeMAC(arg) {
		return ClientView{MacAddress: NormalizeMAC(arg), Name: arg, Sources: []string{"mac"}}, nil
	}
	return ClientView{}, err
}

func matchDevice(devs []map[string]any, arg string) (ClientView, bool) {
	for _, d := range devs {
		name := str(d["name"])
		mac := NormalizeMAC(str(d["mac"]))
		if strings.EqualFold(name, arg) || (LooksLikeMAC(arg) && mac == NormalizeMAC(arg)) {
			return deviceView(d, ""), true
		}
		if LooksLikeIP(arg) {
			for _, ip := range DeviceIPs(d) {
				if ip == arg {
					return deviceView(d, ip), true
				}
			}
		}
	}
	return ClientView{}, false
}

func deviceView(d map[string]any, ip string) ClientView {
	if ip == "" {
		ips := DeviceIPs(d)
		if len(ips) > 0 {
			ip = ips[0]
			for _, candidate := range ips {
				if LooksLikeIP(candidate) && !strings.Contains(candidate, ":") {
					ip = candidate
					break
				}
			}
			// Prefer RFC1918 / lan_ip when present.
			if lan := str(d["lan_ip"]); lan != "" {
				ip = lan
			}
		}
	}
	return ClientView{
		Name:       str(d["name"]),
		MacAddress: NormalizeMAC(str(d["mac"])),
		IPAddress:  ip,
		Type:       "DEVICE",
		Sources:    []string{"stat-device"},
	}
}

func looksLikeClientID(s string) bool {
	if len(s) == 36 {
		return true
	}
	return len(s) == 24
}

func findClientMap(items []map[string]any, arg string, intg *Client) map[string]any {
	var fallback map[string]any
	for _, m := range items {
		view := MergeClientView(nil, m, nil, nil)
		if MatchClientArg(arg, view, m) {
			return m
		}
		if intg != nil && (NormalizeMAC(intg.MacAddress) == view.MacAddress && view.MacAddress != "" || intg.IPAddress != "" && intg.IPAddress == view.IPAddress) {
			fallback = m
		}
	}
	return fallback
}

func FindPorts(devices []map[string]any, clients []map[string]any, mac, ip, name string) []PortView {
	mac, ip, name = strings.TrimSpace(mac), strings.TrimSpace(ip), strings.TrimSpace(name)
	wantMAC := ""
	if mac != "" {
		wantMAC = NormalizeMAC(mac)
	}
	var out []PortView
	clientIndex := map[string][]string{} // deviceMac|port -> client names
	for _, c := range clients {
		v := MergeClientView(nil, c, nil, nil)
		if v.Uplink == nil || v.Uplink.DeviceMAC == "" || v.Uplink.Port == nil {
			continue
		}
		key := v.Uplink.DeviceMAC + "|" + fmt.Sprintf("%d", *v.Uplink.Port)
		label := firstNonEmpty(v.Name, v.IPAddress, v.MacAddress)
		clientIndex[key] = append(clientIndex[key], label)
	}
	for _, d := range devices {
		devName := str(d["name"])
		devMAC := NormalizeMAC(str(d["mac"]))
		raw, _ := d["port_table"].([]any)
		for _, item := range raw {
			p, ok := item.(map[string]any)
			if !ok {
				continue
			}
			view := CompactPort(d, p)
			key := view.DeviceMAC + "|" + fmt.Sprintf("%d", view.Port)
			view.Clients = clientIndex[key]
			if !portMatches(view, d, wantMAC, ip, name, clients, devName, devMAC) {
				continue
			}
			out = append(out, view)
		}
	}
	return out
}

func portMatches(view PortView, device map[string]any, wantMAC, ip, name string, clients []map[string]any, devName, devMAC string) bool {
	if wantMAC == "" && ip == "" && name == "" {
		return true
	}
	if name != "" {
		blob := strings.ToLower(devName + " " + view.Name + " " + strings.Join(view.Clients, " "))
		if !strings.Contains(blob, strings.ToLower(name)) {
			return false
		}
	}
	if wantMAC == "" && ip == "" {
		return true
	}
	if wantMAC != "" && wantMAC == view.DeviceMAC {
		return true
	}
	for _, c := range clients {
		v := MergeClientView(nil, c, nil, nil)
		if v.Uplink == nil || v.Uplink.Port == nil {
			continue
		}
		if NormalizeMAC(v.Uplink.DeviceMAC) != view.DeviceMAC || *v.Uplink.Port != view.Port {
			continue
		}
		if wantMAC != "" && v.MacAddress == wantMAC {
			return true
		}
		if ip != "" && v.IPAddress == ip {
			return true
		}
	}
	return wantMAC == "" && ip == ""
}

type DiagnoseReport struct {
	Client     ClientView     `json:"client"`
	UplinkPath []PathHop      `json:"uplinkPath"`
	AccessPort *PortView      `json:"accessPort,omitempty"`
	Health     []HealthStatus `json:"health"`
}

type HealthStatus struct {
	Subsystem string `json:"subsystem"`
	Status    string `json:"status"`
	Note      string `json:"note,omitempty"`
	Users     any    `json:"users,omitempty"`
	Guests    any    `json:"guests,omitempty"`
}

func AnnotateHealth(items []map[string]any, vpnServers, vpnTunnels int) []HealthStatus {
	out := make([]HealthStatus, 0, len(items))
	for _, h := range items {
		row := HealthStatus{
			Subsystem: str(h["subsystem"]),
			Status:    str(h["status"]),
			Users:     h["num_user"],
			Guests:    h["num_guest"],
		}
		if row.Subsystem == "vpn" && (row.Status == "unknown" || row.Status == "") {
			switch {
			case vpnServers == 0 && vpnTunnels == 0:
				row.Note = "controller reports unknown because no VPN servers or site-to-site tunnels are configured — not an auth or permission error"
			default:
				row.Note = fmt.Sprintf("controller reports unknown despite %d VPN server(s) and %d tunnel(s); metrics are unavailable on this gateway type", vpnServers, vpnTunnels)
			}
		}
		out = append(out, row)
	}
	return out
}
