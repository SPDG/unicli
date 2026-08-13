package selectfields

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Apply projects fields from v using comma-separated paths (e.g. "id,name,page.data").
// Empty select returns v unchanged.
func Apply(v any, selectExpr string) (any, error) {
	selectExpr = strings.TrimSpace(selectExpr)
	if selectExpr == "" {
		return v, nil
	}
	fields := splitFields(selectExpr)
	if len(fields) == 0 {
		return v, nil
	}

	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}

	out := map[string]any{}
	for _, field := range fields {
		val, ok := lookup(root, strings.Split(field, "."))
		if !ok {
			continue
		}
		setPath(out, strings.Split(field, "."), val)
	}
	return out, nil
}

func splitFields(expr string) []string {
	parts := strings.Split(expr, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func lookup(v any, path []string) (any, bool) {
	if len(path) == 0 {
		return v, true
	}
	key := path[0]
	switch cur := v.(type) {
	case map[string]any:
		next, ok := cur[key]
		if !ok {
			return nil, false
		}
		return lookup(next, path[1:])
	case []any:
		// For arrays, project the field from each element when path continues.
		projected := make([]any, 0, len(cur))
		for _, item := range cur {
			val, ok := lookup(item, path)
			if ok {
				projected = append(projected, val)
			}
		}
		return projected, true
	default:
		return nil, false
	}
}

func setPath(dst map[string]any, path []string, val any) {
	if len(path) == 1 {
		dst[path[0]] = val
		return
	}
	child, ok := dst[path[0]].(map[string]any)
	if !ok {
		child = map[string]any{}
		dst[path[0]] = child
	}
	setPath(child, path[1:], val)
}

func MustApply(v any, selectExpr string) any {
	out, err := Apply(v, selectExpr)
	if err != nil {
		panic(fmt.Sprintf("selectfields: %v", err))
	}
	return out
}
