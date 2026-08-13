package network

import (
	"context"
	"encoding/json"
	"net/url"
)

type Network struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	VlanID      int    `json:"vlanId"`
	ZoneID      string `json:"zoneId,omitempty"`
	Management  string `json:"management,omitempty"`
	Default     bool   `json:"default"`
	Subnet      string `json:"subnet,omitempty"`
}

type WifiBroadcast struct {
	ID                         string    `json:"id"`
	Name                       string    `json:"name"`
	Enabled                    bool      `json:"enabled"`
	Type                       string    `json:"type"`
	BroadcastingFrequenciesGHz []float64 `json:"broadcastingFrequenciesGHz,omitempty"`
	SecurityConfiguration      struct {
		Type string `json:"type"`
	} `json:"securityConfiguration"`
}

type FirewallZone struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	NetworkIDs []string `json:"networkIds,omitempty"`
}

// FirewallPolicy is a list-row view.
// Custom (USER_DEFINED) policies often omit `id` in list responses on current
// Network firmware. Built-in SYSTEM_DEFINED policies usually include a UUID.
// Index 2147483647 is UniFi's catch-all ("Allow/Block All") sentinel — the UI
// shows an info icon instead of a numeric ID.
type FirewallPolicy struct {
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	Index       int             `json:"index"`
	Action      json.RawMessage `json:"action,omitempty"`
	Source      json.RawMessage `json:"source,omitempty"`
	Destination json.RawMessage `json:"destination,omitempty"`
	Protocol    json.RawMessage `json:"protocol,omitempty"`
	Metadata    EntityMetadata  `json:"metadata"`
}

type EntityMetadata struct {
	Origin         string `json:"origin,omitempty"`
	Configurable   *bool  `json:"configurable,omitempty"`
}

const CatchAllPolicyIndex = 2147483647

func (p FirewallPolicy) ActionType() string {
	var a struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(p.Action, &a)
	return a.Type
}

type ACLRule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	Index       int             `json:"index"`
	Action      json.RawMessage `json:"action,omitempty"`
	Protocol    json.RawMessage `json:"protocol,omitempty"`
	Source      json.RawMessage `json:"source,omitempty"`
	Destination json.RawMessage `json:"destination,omitempty"`
}

func (a *API) Networks(ctx context.Context, siteID string, offset, limit int) (*Page[Network], error) {
	return getPage[Network](a, ctx, sitePath(siteID, "networks"), offset, limit)
}

func (a *API) Network(ctx context.Context, siteID, networkID string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "networks/"+url.PathEscape(networkID)))
}

func (a *API) WifiBroadcasts(ctx context.Context, siteID string, offset, limit int) (*Page[WifiBroadcast], error) {
	return getPage[WifiBroadcast](a, ctx, sitePath(siteID, "wifi/broadcasts"), offset, limit)
}

func (a *API) WifiBroadcast(ctx context.Context, siteID, wifiID string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "wifi/broadcasts/"+url.PathEscape(wifiID)))
}

func (a *API) FirewallZones(ctx context.Context, siteID string, offset, limit int) (*Page[FirewallZone], error) {
	return getPage[FirewallZone](a, ctx, sitePath(siteID, "firewall/zones"), offset, limit)
}

func (a *API) FirewallZone(ctx context.Context, siteID, zoneID string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "firewall/zones/"+url.PathEscape(zoneID)))
}

func (a *API) FirewallPolicies(ctx context.Context, siteID string, offset, limit int) (*Page[FirewallPolicy], error) {
	return getPage[FirewallPolicy](a, ctx, sitePath(siteID, "firewall/policies"), offset, limit)
}

func (a *API) FirewallPolicy(ctx context.Context, siteID, policyID string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "firewall/policies/"+url.PathEscape(policyID)))
}

func (a *API) ACLRules(ctx context.Context, siteID string, offset, limit int) (*Page[ACLRule], error) {
	return getPage[ACLRule](a, ctx, sitePath(siteID, "acl-rules"), offset, limit)
}

func (a *API) ACLRule(ctx context.Context, siteID, ruleID string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "acl-rules/"+url.PathEscape(ruleID)))
}

func sitePath(siteID, rest string) string {
	return "sites/" + url.PathEscape(siteID) + "/" + rest
}

func getPage[T any](a *API, ctx context.Context, path string, offset, limit int) (*Page[T], error) {
	var out Page[T]
	if err := a.c.GetJSON(ctx, "network", path, pageQuery(offset, limit), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func getObject(a *API, ctx context.Context, path string) (map[string]any, error) {
	var out map[string]any
	if err := a.c.GetJSON(ctx, "network", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
