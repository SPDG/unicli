package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
)

func collectPage[T any](fetch func(offset, limit int) (*network.Page[T], error)) (*network.Page[T], error) {
	return network.Collect(fetch, rootOpts.offset, rootOpts.limit, rootOpts.allPages)
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHex(c) {
				return false
			}
		}
	}
	return true
}

func isHex(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func matchName(got, want string) bool {
	if want == "" {
		return true
	}
	return strings.Contains(strings.ToLower(got), strings.ToLower(want))
}

func parseEnabledFilter() (*bool, error) {
	s := strings.ToLower(strings.TrimSpace(rootOpts.filterEnabled))
	switch s {
	case "":
		return nil, nil
	case "true", "yes", "1", "on":
		v := true
		return &v, nil
	case "false", "no", "0", "off":
		v := false
		return &v, nil
	default:
		return nil, exitf(exitcode.Usage, "invalid --enabled %q (use true or false)", rootOpts.filterEnabled)
	}
}

func resolveID[T any](arg string, items []T, nameOf func(T) string, idOf func(T) string) (string, error) {
	arg = strings.TrimSpace(arg)
	if isUUID(arg) {
		return arg, nil
	}
	want := strings.ToLower(arg)
	var exact, partial []T
	for _, item := range items {
		n := strings.ToLower(strings.TrimSpace(nameOf(item)))
		if n == want {
			exact = append(exact, item)
		} else if n != "" && strings.Contains(n, want) {
			partial = append(partial, item)
		}
	}
	hits := exact
	if len(hits) == 0 {
		hits = partial
	}
	if len(hits) == 0 {
		return arg, nil
	}
	if len(hits) > 1 {
		var names []string
		for _, h := range hits {
			names = append(names, nameOf(h)+" ("+idOf(h)+")")
		}
		return "", exitf(exitcode.Usage, "ambiguous %q: %s", arg, strings.Join(names, ", "))
	}
	id := idOf(hits[0])
	if id == "" {
		return "", exitf(exitcode.NotFound, "%q has no id in the API response", nameOf(hits[0]))
	}
	return id, nil
}

func readJSONBody(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, exitf(exitcode.Usage, "--from-json is required (object, @file, or -)")
	}
	var data []byte
	var err error
	switch {
	case raw == "-":
		data, err = io.ReadAll(os.Stdin)
	case strings.HasPrefix(raw, "@"):
		data, err = os.ReadFile(strings.TrimPrefix(raw, "@"))
	case strings.HasPrefix(raw, "{"):
		data = []byte(raw)
	default:
		data, err = os.ReadFile(raw)
	}
	if err != nil {
		return nil, exitf(exitcode.Usage, "read JSON: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, exitf(exitcode.Usage, "parse JSON: %v", err)
	}
	return body, nil
}

func anyString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return yesNo(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

func rawRows(data []map[string]any, keys ...string) [][]string {
	rows := make([][]string, 0, len(data))
	for _, item := range data {
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = anyString(item[k])
		}
		rows = append(rows, row)
	}
	return rows
}

func filterRaw(data []map[string]any, nameKey string) []map[string]any {
	if rootOpts.filterName == "" {
		return data
	}
	out := make([]map[string]any, 0, len(data))
	for _, item := range data {
		if matchName(anyString(item[nameKey]), rootOpts.filterName) {
			out = append(out, item)
		}
	}
	return out
}
