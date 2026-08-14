package redact

import "encoding/json"

var secretKeys = map[string]struct{}{
	"passphrase":    {},
	"passphrases":   {},
	"password":      {},
	"secret":        {},
	"apikey":        {},
	"api_key":       {},
	"presharedkey":  {},
	"presharedkeys": {},
	"pincode":       {},
	"nfctoken":      {},
}

const placeholder = "[redacted]"

// JSON returns a deep copy of v with known secret fields replaced.
func JSON(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return v
	}
	walk(node)
	return node
}

func walk(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if isSecretKey(k) {
				x[k] = redactValue(val)
				continue
			}
			walk(val)
		}
	case []any:
		for _, item := range x {
			walk(item)
		}
	}
}

func isSecretKey(k string) bool {
	_, ok := secretKeys[normalizeKey(k)]
	return ok
}

func normalizeKey(k string) string {
	b := make([]byte, 0, len(k))
	for i := 0; i < len(k); i++ {
		c := k[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c == '-' || c == '_' {
			continue
		}
		b = append(b, c)
	}
	return string(b)
}

func redactValue(v any) any {
	switch x := v.(type) {
	case string:
		if x == "" {
			return x
		}
		return placeholder
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = redactValue(x[i])
		}
		return out
	case map[string]any:
		walk(x)
		return x
	default:
		if v == nil {
			return v
		}
		return placeholder
	}
}
