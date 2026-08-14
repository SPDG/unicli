package access

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func (a *API) Collection(ctx context.Context, collection string) ([]map[string]any, error) {
	raw, err := a.getRaw(ctx, accessV2(collection))
	if err != nil {
		if isJSONNotFound(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	return parseMapList(raw)
}

func (a *API) Item(ctx context.Context, collection, id string) (map[string]any, error) {
	raw, err := a.getRaw(ctx, accessV2(collection)+"/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	items, err := parseMapList(raw)
	if err != nil {
		return nil, err
	}
	if len(items) == 1 {
		return items[0], nil
	}
	var one map[string]any
	if err := json.Unmarshal(raw, &one); err == nil && len(one) > 0 {
		if _, hasData := one["data"]; !hasData {
			return one, nil
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return items[0], nil
}

func (a *API) LockDoor(ctx context.Context, id string) error {
	path := accessV2("doors") + "/" + url.PathEscape(id) + "/lock"
	return a.c.PutJSON(ctx, path, map[string]any{}, nil)
}

func accessV2(collection string) string {
	collection = strings.Trim(collection, "/")
	return "/proxy/access/api/v2/" + collection
}

func parseMapList(raw json.RawMessage) ([]map[string]any, error) {
	items, err := parseList[map[string]any](raw)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func ItemID(item map[string]any) string {
	for _, k := range []string{"id", "_id", "unique_id"} {
		if v, ok := item[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func ItemName(item map[string]any) string {
	for _, k := range []string{"name", "full_name", "alias"} {
		if v, ok := item[k].(string); ok && v != "" {
			return v
		}
	}
	first, _ := item["first_name"].(string)
	last, _ := item["last_name"].(string)
	return strings.TrimSpace(first + " " + last)
}
