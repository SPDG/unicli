package network

import (
	"context"
	"encoding/json"
	"net/url"
)

const collectPageSize = 100
const collectMaxItems = 10000

func Collect[T any](fetch func(offset, limit int) (*Page[T], error), offset, limit int, all bool) (*Page[T], error) {
	if !all {
		return fetch(offset, limit)
	}
	var data []T
	off := 0
	total := 0
	for {
		page, err := fetch(off, collectPageSize)
		if err != nil {
			return nil, err
		}
		total = page.TotalCount
		data = append(data, page.Data...)
		if len(page.Data) == 0 || len(data) >= total || len(data) >= collectMaxItems {
			break
		}
		off += len(page.Data)
	}
	if data == nil {
		data = []T{}
	}
	return &Page[T]{
		Offset:     0,
		Limit:      len(data),
		Count:      len(data),
		TotalCount: total,
		Data:       data,
	}, nil
}

func DropReadOnly(obj map[string]any, extra ...string) map[string]any {
	raw, err := json.Marshal(obj)
	if err != nil {
		return obj
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return obj
	}
	for _, k := range append([]string{"id", "metadata", "default"}, extra...) {
		delete(out, k)
	}
	return out
}

func (a *API) CreateNetwork(ctx context.Context, siteID string, body any) (map[string]any, error) {
	return postObject(a, ctx, sitePath(siteID, "networks"), body)
}

func (a *API) UpdateNetwork(ctx context.Context, siteID, networkID string, body any) (map[string]any, error) {
	return putObject(a, ctx, sitePath(siteID, "networks/"+url.PathEscape(networkID)), body)
}

func (a *API) DeleteNetwork(ctx context.Context, siteID, networkID string, force bool) error {
	return deleteObject(a, ctx, sitePath(siteID, "networks/"+url.PathEscape(networkID)), force)
}

func (a *API) NetworkReferences(ctx context.Context, siteID, networkID string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "networks/"+url.PathEscape(networkID)+"/references"))
}

func (a *API) CreateWifiBroadcast(ctx context.Context, siteID string, body any) (map[string]any, error) {
	return postObject(a, ctx, sitePath(siteID, "wifi/broadcasts"), body)
}

func (a *API) UpdateWifiBroadcast(ctx context.Context, siteID, wifiID string, body any) (map[string]any, error) {
	return putObject(a, ctx, sitePath(siteID, "wifi/broadcasts/"+url.PathEscape(wifiID)), body)
}

func (a *API) DeleteWifiBroadcast(ctx context.Context, siteID, wifiID string, force bool) error {
	return deleteObject(a, ctx, sitePath(siteID, "wifi/broadcasts/"+url.PathEscape(wifiID)), force)
}

func (a *API) CreateFirewallZone(ctx context.Context, siteID string, body any) (map[string]any, error) {
	return postObject(a, ctx, sitePath(siteID, "firewall/zones"), body)
}

func (a *API) UpdateFirewallZone(ctx context.Context, siteID, zoneID string, body any) (map[string]any, error) {
	return putObject(a, ctx, sitePath(siteID, "firewall/zones/"+url.PathEscape(zoneID)), body)
}

func (a *API) DeleteFirewallZone(ctx context.Context, siteID, zoneID string, force bool) error {
	return deleteObject(a, ctx, sitePath(siteID, "firewall/zones/"+url.PathEscape(zoneID)), force)
}

func (a *API) CreateFirewallPolicy(ctx context.Context, siteID string, body any) (map[string]any, error) {
	return postObject(a, ctx, sitePath(siteID, "firewall/policies"), body)
}

func (a *API) UpdateFirewallPolicy(ctx context.Context, siteID, policyID string, body any) (map[string]any, error) {
	return putObject(a, ctx, sitePath(siteID, "firewall/policies/"+url.PathEscape(policyID)), body)
}

func (a *API) PatchFirewallPolicy(ctx context.Context, siteID, policyID string, body any) (map[string]any, error) {
	return patchObject(a, ctx, sitePath(siteID, "firewall/policies/"+url.PathEscape(policyID)), body)
}

func (a *API) DeleteFirewallPolicy(ctx context.Context, siteID, policyID string, force bool) error {
	return deleteObject(a, ctx, sitePath(siteID, "firewall/policies/"+url.PathEscape(policyID)), force)
}

func (a *API) CreateACLRule(ctx context.Context, siteID string, body any) (map[string]any, error) {
	return postObject(a, ctx, sitePath(siteID, "acl-rules"), body)
}

func (a *API) UpdateACLRule(ctx context.Context, siteID, ruleID string, body any) (map[string]any, error) {
	return putObject(a, ctx, sitePath(siteID, "acl-rules/"+url.PathEscape(ruleID)), body)
}

func (a *API) DeleteACLRule(ctx context.Context, siteID, ruleID string, force bool) error {
	return deleteObject(a, ctx, sitePath(siteID, "acl-rules/"+url.PathEscape(ruleID)), force)
}

func postObject(a *API, ctx context.Context, path string, body any) (map[string]any, error) {
	var out map[string]any
	if err := a.c.PostJSON(ctx, "network", path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func putObject(a *API, ctx context.Context, path string, body any) (map[string]any, error) {
	var out map[string]any
	if err := a.c.PutAppJSON(ctx, "network", path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func patchObject(a *API, ctx context.Context, path string, body any) (map[string]any, error) {
	var out map[string]any
	if err := a.c.PatchJSON(ctx, "network", path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func deleteObject(a *API, ctx context.Context, path string, force bool) error {
	q := url.Values{}
	if force {
		q.Set("force", "true")
	}
	return a.c.DeleteJSON(ctx, "network", path, q)
}
