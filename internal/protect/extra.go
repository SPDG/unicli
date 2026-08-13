package protect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

func (a *API) NVR(ctx context.Context) (map[string]any, error) {
	var raw json.RawMessage
	if err := a.c.GetJSON(ctx, "protect", "nvrs", nil, &raw); err != nil {
		return nil, err
	}
	return decodeOne(raw)
}

func (a *API) Devices(ctx context.Context, collection string) ([]map[string]any, error) {
	var raw json.RawMessage
	if err := a.c.GetJSON(ctx, "protect", collection, nil, &raw); err != nil {
		return nil, err
	}
	return decodeList(raw)
}

func (a *API) Device(ctx context.Context, collection, id string) (map[string]any, error) {
	var raw json.RawMessage
	path := collection + "/" + url.PathEscape(id)
	if err := a.c.GetJSON(ctx, "protect", path, nil, &raw); err != nil {
		return nil, err
	}
	return decodeOne(raw)
}

func (a *API) Snapshot(ctx context.Context, cameraID string) ([]byte, error) {
	path := "cameras/" + url.PathEscape(cameraID) + "/snapshot"
	return a.c.GetBytes(ctx, "protect", path, nil)
}

func (a *API) RTSPS(ctx context.Context, cameraID string) (map[string]any, error) {
	var out map[string]any
	path := "cameras/" + url.PathEscape(cameraID) + "/rtsps-stream"
	if err := a.c.GetJSON(ctx, "protect", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *API) RestartCamera(ctx context.Context, cameraID string) error {
	path := "cameras/" + url.PathEscape(cameraID) + "/restart"
	return a.c.PostJSON(ctx, "protect", path, map[string]any{}, nil)
}

func decodeList(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []map[string]any{}, nil
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	one, err := decodeOne(raw)
	if err != nil {
		return nil, err
	}
	return []map[string]any{one}, nil
}

func decodeOne(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("empty protect payload")
	}
	var one map[string]any
	if err := json.Unmarshal(raw, &one); err == nil && len(one) > 0 {
		return one, nil
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return list[0], nil
	}
	return nil, fmt.Errorf("decode protect payload")
}

func DeviceID(item map[string]any) string {
	if v, ok := item["id"].(string); ok && v != "" {
		return v
	}
	if v, ok := item["_id"].(string); ok {
		return v
	}
	return ""
}

func DeviceName(item map[string]any) string {
	if v, ok := item["name"].(string); ok {
		return v
	}
	return ""
}
