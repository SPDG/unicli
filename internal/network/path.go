package network

import (
	"strconv"
	"strings"
)

type Topology struct {
	Vertices          []TopologyVertex `json:"vertices"`
	Edges             []TopologyEdge   `json:"edges"`
	HasUnknownSwitch  bool             `json:"hasUnknownSwitch,omitempty"`
}

type TopologyVertex struct {
	Type  string `json:"type"`
	MAC   string `json:"mac"`
	Name  string `json:"name,omitempty"`
	Model string `json:"model,omitempty"`
	State any    `json:"state,omitempty"`
}

type TopologyEdge struct {
	Type             string `json:"type"`
	UplinkMAC        string `json:"uplinkMac"`
	DownlinkMAC      string `json:"downlinkMac"`
	UplinkPortNumber *int   `json:"uplinkPortNumber,omitempty"`
	RateMbps         *int   `json:"rateMbps,omitempty"`
	NetworkID        string `json:"networkId,omitempty"`
	ESSID            string `json:"essid,omitempty"`
	RadioBand        string `json:"radioBand,omitempty"`
	Channel          *int   `json:"channel,omitempty"`
}

type PathHop struct {
	MAC          string    `json:"mac"`
	Name         string    `json:"name,omitempty"`
	Kind         string    `json:"kind,omitempty"`
	Model        string    `json:"model,omitempty"`
	Uplink       *PathLink `json:"uplink,omitempty"`
	Attachment   string    `json:"attachment,omitempty"`
	Note         string    `json:"note,omitempty"`
	SharedOnPort *int      `json:"sharedOnPort,omitempty"`
	SharedCount  *int      `json:"sharedCount,omitempty"`
}

type PathLink struct {
	Type      string   `json:"type,omitempty"`
	RateMbps  *int     `json:"rateMbps,omitempty"`
	Port      *int     `json:"port,omitempty"`
	PortName  string   `json:"portName,omitempty"`
	Device    string   `json:"device,omitempty"`
	DeviceMAC string   `json:"deviceMac,omitempty"`
	LAG       *LAGView `json:"lag,omitempty"`
	ESSID     string   `json:"essid,omitempty"`
	RadioBand string   `json:"radioBand,omitempty"`
}

type TraceResult struct {
	From     ClientView `json:"from"`
	To       ClientView `json:"to"`
	Found    bool       `json:"found"`
	Complete bool       `json:"complete"`
	Hops     []PathHop  `json:"hops"`
	Notes    []string   `json:"notes,omitempty"`
}

const (
	AttachmentPhysical = "physical"
	AttachmentVirtual  = "virtual-behind-host"
	AttachmentUnknown  = "unknown"
)

func TracePath(topo Topology, devices []map[string]any, srcMAC, dstMAC string) TraceResult {
	srcMAC, dstMAC = NormalizeMAC(srcMAC), NormalizeMAC(dstMAC)
	names := vertexIndex(topo)
	adj := buildAdj(topo)
	path := bfsMAC(adj, srcMAC, dstMAC)
	out := TraceResult{Found: len(path) > 0}
	if !out.Found {
		return out
	}
	shared := sharedAccessCounts(topo)
	hops := make([]PathHop, 0, len(path))
	for i, mac := range path {
		hop := hopFromMAC(mac, names, devices)
		if i > 0 {
			e := adj[path[i-1]][mac]
			hop.Uplink = linkView(e, devices, NormalizeMAC(e.UplinkMAC))
		} else if ae, ok := accessEdge(topo, mac); ok {
			hop.Uplink = linkView(ae, devices, NormalizeMAC(ae.UplinkMAC))
		}
		annotateHop(&hop, shared)
		hops = append(hops, hop)
	}
	out.Hops = hops
	out.Complete, out.Notes = pathCompleteness(hops)
	return out
}

func UplinkChain(topo Topology, devices []map[string]any, startMAC string) []PathHop {
	startMAC = NormalizeMAC(startMAC)
	names := vertexIndex(topo)
	up := map[string]TopologyEdge{}
	for _, e := range topo.Edges {
		up[NormalizeMAC(e.DownlinkMAC)] = e
	}
	seen := map[string]bool{}
	var hops []PathHop
	cur := startMAC
	shared := sharedAccessCounts(topo)
	for cur != "" && !seen[cur] {
		seen[cur] = true
		hop := hopFromMAC(cur, names, devices)
		e, ok := up[cur]
		if ok {
			hop.Uplink = linkView(e, devices, NormalizeMAC(e.UplinkMAC))
		}
		annotateHop(&hop, shared)
		hops = append(hops, hop)
		if !ok {
			break
		}
		cur = NormalizeMAC(e.UplinkMAC)
	}
	return hops
}

func hopFromMAC(mac string, names map[string]TopologyVertex, devices []map[string]any) PathHop {
	v := names[mac]
	hop := PathHop{MAC: mac, Name: v.Name, Kind: v.Type, Model: v.Model, Attachment: AttachmentPhysical}
	if hop.Name == "" {
		if d := findDeviceByMAC(devices, mac); d != nil {
			hop.Name = str(d["name"])
			hop.Model = str(d["model"])
			hop.Kind = "DEVICE"
		}
	}
	return hop
}

func sharedAccessCounts(topo Topology) map[string]int {
	out := map[string]int{}
	for _, e := range topo.Edges {
		if e.UplinkMAC == "" || e.UplinkPortNumber == nil {
			continue
		}
		key := NormalizeMAC(e.UplinkMAC) + "|" + strconv.Itoa(*e.UplinkPortNumber)
		out[key]++
	}
	return out
}

func annotateHop(hop *PathHop, shared map[string]int) {
	if hop.Uplink != nil && hop.Uplink.DeviceMAC != "" && hop.Uplink.Port != nil {
		n := shared[hop.Uplink.DeviceMAC+"|"+strconv.Itoa(*hop.Uplink.Port)]
		if n > 1 && (hop.Kind == "CLIENT" || hop.Kind == "") {
			hop.Attachment = AttachmentVirtual
			hop.SharedOnPort = hop.Uplink.Port
			hop.SharedCount = &n
			hop.Note = "virtual endpoint behind host / unknown physical attachment; UniFi only sees a shared switch port"
			if hop.Uplink.Device == "" {
				hop.Uplink.Device = hop.Uplink.DeviceMAC
			}
		}
	}
	if hop.Name == "" && hop.Kind != "CLIENT" {
		if hop.Attachment != AttachmentVirtual {
			hop.Attachment = AttachmentUnknown
			hop.Note = "unknown physical attachment; MAC is not an adopted UniFi device"
		}
	}
}

func pathCompleteness(hops []PathHop) (bool, []string) {
	complete := true
	var notes []string
	for _, h := range hops {
		if h.Attachment == AttachmentVirtual {
			complete = false
			notes = appendUnique(notes, h.Note)
		}
		if h.Attachment == AttachmentUnknown {
			complete = false
			notes = appendUnique(notes, h.Note)
		}
	}
	return complete, notes
}

func accessEdge(topo Topology, mac string) (TopologyEdge, bool) {
	mac = NormalizeMAC(mac)
	for _, e := range topo.Edges {
		if NormalizeMAC(e.DownlinkMAC) == mac {
			return e, true
		}
	}
	return TopologyEdge{}, false
}

func linkView(e TopologyEdge, devices []map[string]any, uplinkMAC string) *PathLink {
	link := &PathLink{
		Type:      e.Type,
		RateMbps:  e.RateMbps,
		Port:      e.UplinkPortNumber,
		DeviceMAC: NormalizeMAC(uplinkMAC),
		ESSID:     e.ESSID,
		RadioBand: e.RadioBand,
	}
	if d := findDeviceByMAC(devices, uplinkMAC); d != nil {
		link.Device = str(d["name"])
		if link.DeviceMAC == "" {
			link.DeviceMAC = NormalizeMAC(str(d["mac"]))
		}
		if e.UplinkPortNumber != nil {
			if p := findPort(d, *e.UplinkPortNumber); p != nil {
				link.PortName = str(p["name"])
				link.LAG = lagFromPort(p)
				if link.RateMbps == nil {
					link.RateMbps = asIntPtr(p["speed"])
				}
			}
		}
	}
	return link
}

func vertexIndex(topo Topology) map[string]TopologyVertex {
	out := map[string]TopologyVertex{}
	for _, v := range topo.Vertices {
		out[NormalizeMAC(v.MAC)] = v
	}
	return out
}

func buildAdj(topo Topology) map[string]map[string]TopologyEdge {
	adj := map[string]map[string]TopologyEdge{}
	add := func(a, b string, e TopologyEdge) {
		a, b = NormalizeMAC(a), NormalizeMAC(b)
		if a == "" || b == "" {
			return
		}
		if adj[a] == nil {
			adj[a] = map[string]TopologyEdge{}
		}
		adj[a][b] = e
	}
	for _, e := range topo.Edges {
		add(e.DownlinkMAC, e.UplinkMAC, e)
		add(e.UplinkMAC, e.DownlinkMAC, e)
	}
	return adj
}

func bfsMAC(adj map[string]map[string]TopologyEdge, src, dst string) []string {
	if src == "" || dst == "" {
		return nil
	}
	if src == dst {
		return []string{src}
	}
	prev := map[string]string{}
	q := []string{src}
	seen := map[string]bool{src: true}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		for nb := range adj[cur] {
			if seen[nb] {
				continue
			}
			seen[nb] = true
			prev[nb] = cur
			if nb == dst {
				return reconstruct(prev, src, dst)
			}
			q = append(q, nb)
		}
	}
	return nil
}

func reconstruct(prev map[string]string, src, dst string) []string {
	var rev []string
	for cur := dst; cur != ""; cur = prev[cur] {
		rev = append(rev, cur)
		if cur == src {
			break
		}
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if len(rev) == 0 || rev[0] != src {
		return nil
	}
	return rev
}

func LooksLikeMAC(s string) bool {
	n := NormalizeMAC(s)
	return len(n) == 17 && strings.Count(n, ":") == 5
}
