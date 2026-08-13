package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Legacy controller (non-Integration) Network API. Same X-API-KEY as Integration,
// but the site path is internalReference (usually "default"), not the site UUID.
// Replace these helpers when Ubiquiti exposes the same resources on Integration.

type legacyEnvelope struct {
	Meta struct {
		RC  string `json:"rc"`
		Msg string `json:"msg"`
	} `json:"meta"`
	Data json.RawMessage `json:"data"`
}

func (a *API) LegacySiteSlug(ctx context.Context, preferred string) (string, error) {
	page, err := a.Sites(ctx, 0, 25)
	if err != nil {
		return "", err
	}
	if len(page.Data) == 0 {
		return "", fmt.Errorf("no sites found")
	}
	preferred = strings.TrimSpace(preferred)
	if preferred == "" {
		return firstSlug(page.Data[0]), nil
	}
	for _, s := range page.Data {
		if s.ID == preferred || s.InternalRef == preferred || strings.EqualFold(s.Name, preferred) {
			return firstSlug(s), nil
		}
	}
	return preferred, nil
}

func firstSlug(s Site) string {
	if s.InternalRef != "" {
		return s.InternalRef
	}
	return "default"
}

func (a *API) LegacyList(ctx context.Context, siteSlug, collection string) ([]map[string]any, error) {
	var env legacyEnvelope
	path := legacyREST(siteSlug, collection)
	if err := a.c.GetPathJSON(ctx, path, nil, &env); err != nil {
		return nil, err
	}
	if err := env.check(); err != nil {
		return nil, err
	}
	return decodeLegacyList(env.Data)
}

func (a *API) LegacyGet(ctx context.Context, siteSlug, collection, id string) (map[string]any, error) {
	var env legacyEnvelope
	path := legacyREST(siteSlug, collection) + "/" + url.PathEscape(id)
	if err := a.c.GetPathJSON(ctx, path, nil, &env); err != nil {
		return nil, err
	}
	if err := env.check(); err != nil {
		return nil, err
	}
	items, err := decodeLegacyList(env.Data)
	if err != nil {
		return nil, err
	}
	if len(items) == 1 {
		return items[0], nil
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return items[0], nil
}

func (a *API) LegacyCreate(ctx context.Context, siteSlug, collection string, body any) (map[string]any, error) {
	var env legacyEnvelope
	if err := a.c.PostPathJSON(ctx, legacyREST(siteSlug, collection), body, &env); err != nil {
		return nil, err
	}
	if err := env.check(); err != nil {
		return nil, err
	}
	items, err := decodeLegacyList(env.Data)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return map[string]any{"status": "ok"}, nil
	}
	return items[0], nil
}

func (a *API) LegacyUpdate(ctx context.Context, siteSlug, collection, id string, body any) (map[string]any, error) {
	var env legacyEnvelope
	path := legacyREST(siteSlug, collection) + "/" + url.PathEscape(id)
	if err := a.c.PutJSON(ctx, path, body, &env); err != nil {
		return nil, err
	}
	if err := env.check(); err != nil {
		return nil, err
	}
	items, err := decodeLegacyList(env.Data)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return map[string]any{"status": "ok", "_id": id}, nil
	}
	return items[0], nil
}

func (a *API) LegacyDelete(ctx context.Context, siteSlug, collection, id string) error {
	var env legacyEnvelope
	path := legacyREST(siteSlug, collection) + "/" + url.PathEscape(id)
	if err := a.c.DeletePathJSON(ctx, path, nil); err != nil {
		return err
	}
	_ = env
	return nil
}

func (a *API) TrafficRoutes(ctx context.Context, siteSlug string) ([]map[string]any, error) {
	var raw json.RawMessage
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/trafficroutes"
	if err := a.c.GetPathJSON(ctx, path, nil, &raw); err != nil {
		return nil, err
	}
	return decodeLegacyList(raw)
}

func (a *API) CreateTrafficRoute(ctx context.Context, siteSlug string, body any) (map[string]any, error) {
	var raw json.RawMessage
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/trafficroutes"
	if err := a.c.PostPathJSON(ctx, path, body, &raw); err != nil {
		return nil, err
	}
	items, err := decodeLegacyList(raw)
	if err != nil || len(items) == 0 {
		return map[string]any{"status": "ok"}, err
	}
	return items[0], nil
}

func (a *API) UpdateTrafficRoute(ctx context.Context, siteSlug, id string, body any) (map[string]any, error) {
	var raw json.RawMessage
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/trafficroutes/" + url.PathEscape(id)
	if err := a.c.PutJSON(ctx, path, body, &raw); err != nil {
		return nil, err
	}
	items, err := decodeLegacyList(raw)
	if err != nil || len(items) == 0 {
		return map[string]any{"status": "ok", "id": id}, err
	}
	return items[0], nil
}

func (a *API) DeleteTrafficRoute(ctx context.Context, siteSlug, id string) error {
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/trafficroutes/" + url.PathEscape(id)
	return a.c.DeletePathJSON(ctx, path, nil)
}

func (e legacyEnvelope) check() error {
	if e.Meta.RC == "" || e.Meta.RC == "ok" {
		return nil
	}
	msg := e.Meta.Msg
	if msg == "" {
		msg = e.Meta.RC
	}
	return fmt.Errorf("legacy API: %s", msg)
}

func legacyREST(siteSlug, collection string) string {
	return "/proxy/network/api/s/" + url.PathEscape(siteSlug) + "/rest/" + strings.TrimPrefix(collection, "/")
}

func decodeLegacyList(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []map[string]any{}, nil
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	var one map[string]any
	if err := json.Unmarshal(raw, &one); err == nil {
		return []map[string]any{one}, nil
	}
	return nil, fmt.Errorf("decode legacy payload")
}

func (a *API) StatDevices(ctx context.Context, siteSlug string) ([]map[string]any, error) {
	return a.legacyStat(ctx, siteSlug, "device")
}

func (a *API) StatHealth(ctx context.Context, siteSlug string) ([]map[string]any, error) {
	return a.legacyStat(ctx, siteSlug, "health")
}

func (a *API) StatSysinfo(ctx context.Context, siteSlug string) (map[string]any, error) {
	items, err := a.legacyStat(ctx, siteSlug, "sysinfo")
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return map[string]any{}, nil
	}
	return items[0], nil
}

func (a *API) legacyStat(ctx context.Context, siteSlug, kind string) ([]map[string]any, error) {
	var env legacyEnvelope
	path := "/proxy/network/api/s/" + url.PathEscape(siteSlug) + "/stat/" + kind
	if err := a.c.GetPathJSON(ctx, path, nil, &env); err != nil {
		return nil, err
	}
	if err := env.check(); err != nil {
		return nil, err
	}
	return decodeLegacyList(env.Data)
}

func (a *API) UpdateLegacyDevice(ctx context.Context, siteSlug, deviceID string, body any) (map[string]any, error) {
	return a.LegacyUpdate(ctx, siteSlug, "device", deviceID, body)
}

func (a *API) V2FirewallPolicies(ctx context.Context, siteSlug string) ([]map[string]any, error) {
	var raw json.RawMessage
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/firewall-policies"
	if err := a.c.GetPathJSON(ctx, path, nil, &raw); err != nil {
		return nil, err
	}
	return decodeLegacyList(raw)
}

func (a *API) V2FirewallPolicy(ctx context.Context, siteSlug, id string) (map[string]any, error) {
	var raw json.RawMessage
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/firewall-policies/" + url.PathEscape(id)
	if err := a.c.GetPathJSON(ctx, path, nil, &raw); err != nil {
		return nil, err
	}
	items, err := decodeLegacyList(raw)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		var one map[string]any
		if err := json.Unmarshal(raw, &one); err == nil && len(one) > 0 {
			return one, nil
		}
		return nil, fmt.Errorf("not found")
	}
	return items[0], nil
}

func (a *API) UpdateV2FirewallPolicy(ctx context.Context, siteSlug, id string, body any) (map[string]any, error) {
	var raw json.RawMessage
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/firewall-policies/" + url.PathEscape(id)
	if err := a.c.PutJSON(ctx, path, body, &raw); err != nil {
		return nil, err
	}
	items, err := decodeLegacyList(raw)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return map[string]any{"status": "ok", "_id": id}, nil
	}
	return items[0], nil
}

func (a *API) DeleteV2FirewallPolicy(ctx context.Context, siteSlug, id string) error {
	path := "/proxy/network/v2/api/site/" + url.PathEscape(siteSlug) + "/firewall-policies/" + url.PathEscape(id)
	return a.c.DeletePathJSON(ctx, path, nil)
}

func SlicePage(data []map[string]any, offset, limit int, all bool) *Page[map[string]any] {
	total := len(data)
	if all {
		return &Page[map[string]any]{Offset: 0, Limit: total, Count: total, TotalCount: total, Data: data}
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 25
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return &Page[map[string]any]{
		Offset: offset, Limit: limit, Count: end - offset, TotalCount: total, Data: data[offset:end],
	}
}

func LegacyID(item map[string]any) string {
	if v, ok := item["_id"].(string); ok && v != "" {
		return v
	}
	if v, ok := item["id"].(string); ok {
		return v
	}
	return ""
}
