package network

import (
	"encoding/json"
	"net"
	"strings"
	"time"
)

// ClientView is a backend-stable client record for agents.
type ClientView struct {
	ID             string      `json:"id,omitempty"`
	Name           string      `json:"name,omitempty"`
	Type           string      `json:"type,omitempty"`
	MacAddress     string      `json:"macAddress,omitempty"`
	IPAddress      string      `json:"ipAddress,omitempty"`
	VLAN           *int        `json:"vlan,omitempty"`
	Network        string      `json:"network,omitempty"`
	NetworkID      string      `json:"networkId,omitempty"`
	Status         string      `json:"status,omitempty"`
	Authorized     *bool       `json:"authorized,omitempty"`
	Blocked        *bool       `json:"blocked,omitempty"`
	Guest          *bool       `json:"guest,omitempty"`
	ConnectedAt    string      `json:"connectedAt,omitempty"`
	LastSeen       string      `json:"lastSeen,omitempty"`
	UptimeSec      *int64      `json:"uptimeSec,omitempty"`
	LinkSpeedMbps  *int        `json:"linkSpeedMbps,omitempty"`
	Uplink         *UplinkView `json:"uplink,omitempty"`
	Sources        []string    `json:"sources,omitempty"`
}

type UplinkView struct {
	Device        string   `json:"device,omitempty"`
	DeviceMAC     string   `json:"deviceMac,omitempty"`
	Port          *int     `json:"port,omitempty"`
	PortName      string   `json:"portName,omitempty"`
	SpeedMbps     *int     `json:"speedMbps,omitempty"`
	Up            *bool    `json:"up,omitempty"`
	Media         string   `json:"media,omitempty"`
	LAG           *LAGView `json:"lag,omitempty"`
	RXErrors      *int64   `json:"rxErrors,omitempty"`
	TXErrors      *int64   `json:"txErrors,omitempty"`
	RXDropped     *int64   `json:"rxDropped,omitempty"`
	TXDropped     *int64   `json:"txDropped,omitempty"`
	LinkDownCount *int64   `json:"linkDownCount,omitempty"`
	STPChanges    *int64   `json:"stpStateChangeCount,omitempty"`
}

type LAGView struct {
	Index    *int  `json:"index,omitempty"`
	Member   bool  `json:"member"`
	Members  []int `json:"members,omitempty"`
	AggBy    *int  `json:"aggregatedBy,omitempty"`
	NumPorts *int  `json:"numPorts,omitempty"`
}

type PortView struct {
	Device     string   `json:"device,omitempty"`
	DeviceMAC  string   `json:"deviceMac,omitempty"`
	DeviceID   string   `json:"deviceId,omitempty"`
	Port       int      `json:"port"`
	Name       string   `json:"name,omitempty"`
	Up         *bool    `json:"up,omitempty"`
	SpeedMbps  *int     `json:"speedMbps,omitempty"`
	PoE        string   `json:"poe,omitempty"`
	Media      string   `json:"media,omitempty"`
	IsUplink   *bool    `json:"isUplink,omitempty"`
	LAG        *LAGView `json:"lag,omitempty"`
	RXErrors   *int64   `json:"rxErrors,omitempty"`
	TXErrors   *int64   `json:"txErrors,omitempty"`
	RXDropped  *int64   `json:"rxDropped,omitempty"`
	TXDropped  *int64   `json:"txDropped,omitempty"`
	LinkDownCount *int64 `json:"linkDownCount,omitempty"`
	Clients    []string `json:"clients,omitempty"`
}

func LooksLikeIP(s string) bool {
	return net.ParseIP(strings.TrimSpace(s)) != nil
}

func DeviceIPs(d map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, k := range []string{"ip", "lan_ip", "ipAddress"} {
		add(str(d[k]))
	}
	if nt, ok := d["network_table"].([]any); ok {
		for _, item := range nt {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			add(str(m["ip"]))
		}
	}
	return out
}

func NormalizeMAC(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'F' {
			c += 'a' - 'A'
		}
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			b.WriteByte(c)
		}
	}
	h := b.String()
	if len(h) != 12 {
		return strings.ToLower(strings.TrimSpace(s))
	}
	return h[0:2] + ":" + h[2:4] + ":" + h[4:6] + ":" + h[6:8] + ":" + h[8:10] + ":" + h[10:12]
}

func CompactPort(device map[string]any, port map[string]any) PortView {
	idx := asInt(port["port_idx"])
	view := PortView{
		Device:        str(device["name"]),
		DeviceMAC:     NormalizeMAC(str(device["mac"])),
		DeviceID:      LegacyID(device),
		Port:          idx,
		Name:          str(port["name"]),
		PoE:           str(port["poe_mode"]),
		Media:         str(port["media"]),
		Up:            asBoolPtr(port["up"]),
		SpeedMbps:     asIntPtr(port["speed"]),
		IsUplink:      asBoolPtr(port["is_uplink"]),
		RXErrors:      asInt64Ptr(port["rx_errors"]),
		TXErrors:      asInt64Ptr(port["tx_errors"]),
		RXDropped:     asInt64Ptr(port["rx_dropped"]),
		TXDropped:     asInt64Ptr(port["tx_dropped"]),
		LinkDownCount: asInt64Ptr(port["link_down_count"]),
		LAG:           lagFromPort(port),
	}
	if view.Device == "" {
		view.Device = view.DeviceMAC
	}
	return view
}

func lagFromPort(port map[string]any) *LAGView {
	member := asBool(port["lag_member"])
	aggBy := asIntPtr(port["aggregated_by"])
	idx := asIntPtr(port["lag_idx"])
	n := asIntPtr(port["aggregate_num_ports"])
	var members []int
	if raw, ok := port["aggregate_members"].([]any); ok {
		for _, v := range raw {
			members = append(members, asInt(v))
		}
	}
	if !member && aggBy == nil && idx == nil && n == nil && len(members) == 0 {
		return nil
	}
	return &LAGView{Index: idx, Member: member, Members: members, AggBy: aggBy, NumPorts: n}
}

func MergeClientView(intg *Client, active, sta map[string]any, devices []map[string]any) ClientView {
	v := ClientView{Sources: nil}
	if intg != nil {
		v.ID = intg.ID
		v.Name = intg.Name
		v.Type = intg.Type
		v.MacAddress = NormalizeMAC(intg.MacAddress)
		v.IPAddress = intg.IPAddress
		v.ConnectedAt = intg.ConnectedAt
		v.Sources = append(v.Sources, "integration")
	}
	applyClientMap(&v, active, "v2-active")
	applyClientMap(&v, sta, "stat-sta")
	if v.Uplink != nil && v.Uplink.DeviceMAC != "" {
		dev := findDeviceByMAC(devices, v.Uplink.DeviceMAC)
		if dev != nil && v.Uplink.Port != nil {
			port := findPort(dev, *v.Uplink.Port)
			if port != nil {
				cp := CompactPort(dev, port)
				v.Uplink.Device = cp.Device
				v.Uplink.PortName = cp.Name
				v.Uplink.SpeedMbps = cp.SpeedMbps
				v.Uplink.Up = cp.Up
				v.Uplink.Media = cp.Media
				v.Uplink.LAG = cp.LAG
				v.Uplink.RXErrors = cp.RXErrors
				v.Uplink.TXErrors = cp.TXErrors
				v.Uplink.RXDropped = cp.RXDropped
				v.Uplink.TXDropped = cp.TXDropped
				v.Uplink.LinkDownCount = cp.LinkDownCount
				v.Uplink.STPChanges = asInt64Ptr(port["stp_state_change_count"])
				v.Sources = appendUnique(v.Sources, "port-table")
			} else if v.Uplink.Device == "" {
				v.Uplink.Device = str(dev["name"])
			}
		}
	}
	return v
}

func applyClientMap(v *ClientView, m map[string]any, source string) {
	if m == nil {
		return
	}
	v.Sources = appendUnique(v.Sources, source)
	if id := str(m["id"]); id != "" && v.ID == "" {
		v.ID = id
	}
	if id := str(m["user_id"]); id != "" && v.ID == "" {
		v.ID = id
	}
	name := firstNonEmpty(str(m["display_name"]), str(m["name"]), str(m["hostname"]))
	if name != "" {
		v.Name = name
	}
	if t := clientType(m); t != "" {
		v.Type = t
	}
	if mac := NormalizeMAC(str(m["mac"])); mac != "" {
		v.MacAddress = mac
	}
	if ip := firstNonEmpty(str(m["ip"]), str(m["ipAddress"])); ip != "" {
		v.IPAddress = ip
	}
	if vlan := asIntPtr(m["vlan"]); vlan != nil {
		v.VLAN = vlan
	}
	if netn := firstNonEmpty(str(m["network_name"]), str(m["network"])); netn != "" {
		v.Network = netn
	}
	if nid := firstNonEmpty(str(m["network_id"]), str(m["networkId"])); nid != "" {
		v.NetworkID = nid
	}
	if st := str(m["status"]); st != "" {
		v.Status = st
	}
	if b := asBoolPtr(m["authorized"]); b != nil {
		v.Authorized = b
	}
	if b := asBoolPtr(m["blocked"]); b != nil {
		v.Blocked = b
	}
	if b := asBoolPtr(m["is_guest"]); b != nil {
		v.Guest = b
	}
	if up := asInt64Ptr(m["uptime"]); up != nil {
		v.UptimeSec = up
	}
	if spd := asIntPtr(m["wired_rate_mbps"]); spd != nil {
		v.LinkSpeedMbps = spd
	}
	if ls := unixOrRFC3339(m["last_seen"]); ls != "" {
		v.LastSeen = ls
	}
	uplinkMAC := firstNonEmpty(str(m["uplink_mac"]), str(m["sw_mac"]), str(m["last_uplink_mac"]))
	port := firstIntPtr(m["sw_port"], m["last_uplink_remote_port"])
	if uplinkMAC != "" || port != nil {
		if v.Uplink == nil {
			v.Uplink = &UplinkView{}
		}
		if uplinkMAC != "" {
			v.Uplink.DeviceMAC = NormalizeMAC(uplinkMAC)
		}
		if name := str(m["last_uplink_name"]); name != "" {
			v.Uplink.Device = name
		}
		if port != nil {
			v.Uplink.Port = port
		}
	}
}

func clientType(m map[string]any) string {
	if t := str(m["type"]); t != "" {
		return t
	}
	if b, ok := m["is_wired"].(bool); ok {
		if b {
			return "WIRED"
		}
		return "WIRELESS"
	}
	return ""
}

func findDeviceByMAC(devices []map[string]any, mac string) map[string]any {
	want := NormalizeMAC(mac)
	for _, d := range devices {
		if NormalizeMAC(str(d["mac"])) == want {
			return d
		}
	}
	return nil
}

func findPort(device map[string]any, idx int) map[string]any {
	raw, _ := device["port_table"].([]any)
	for _, item := range raw {
		p, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if asInt(p["port_idx"]) == idx {
			return p
		}
	}
	return nil
}

func MatchClientArg(arg string, v ClientView, raw map[string]any) bool {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return false
	}
	if strings.EqualFold(v.ID, arg) || strings.EqualFold(v.Name, arg) || strings.EqualFold(v.IPAddress, arg) {
		return true
	}
	if NormalizeMAC(arg) != "" && NormalizeMAC(arg) == v.MacAddress {
		return true
	}
	if raw != nil {
		for _, k := range []string{"id", "user_id", "mac", "ip", "name", "display_name", "hostname"} {
			if strings.EqualFold(str(raw[k]), arg) {
				return true
			}
			if k == "mac" && NormalizeMAC(arg) == NormalizeMAC(str(raw[k])) {
				return true
			}
		}
	}
	return strings.Contains(strings.ToLower(v.Name), strings.ToLower(arg))
}

func PortViewsAsMaps(views []PortView) []map[string]any {
	raw, err := json.Marshal(views)
	if err != nil {
		return nil
	}
	var out []map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	default:
		return ""
	}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func appendUnique(in []string, s string) []string {
	for _, x := range in {
		if x == s {
			return in
		}
	}
	return append(in, s)
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func asBoolPtr(v any) *bool {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case bool:
		return &x
	default:
		return nil
	}
}

func asInt(v any) int {
	if p := asIntPtr(v); p != nil {
		return *p
	}
	return 0
}

func asIntPtr(v any) *int {
	switch x := v.(type) {
	case int:
		return &x
	case int32:
		n := int(x)
		return &n
	case int64:
		n := int(x)
		return &n
	case float64:
		n := int(x)
		return &n
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return nil
		}
		n := int(i)
		return &n
	case bool:
		if x {
			n := 1
			return &n
		}
		return nil
	default:
		return nil
	}
}

func firstIntPtr(vs ...any) *int {
	for _, v := range vs {
		if p := asIntPtr(v); p != nil {
			return p
		}
	}
	return nil
}

func asInt64Ptr(v any) *int64 {
	switch x := v.(type) {
	case int:
		n := int64(x)
		return &n
	case int64:
		return &x
	case float64:
		n := int64(x)
		return &n
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return nil
		}
		return &i
	default:
		return nil
	}
}

func unixOrRFC3339(v any) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	if p := asInt64Ptr(v); p != nil && *p > 0 {
		return time.Unix(*p, 0).UTC().Format(time.RFC3339)
	}
	return ""
}
