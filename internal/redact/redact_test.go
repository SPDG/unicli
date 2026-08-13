package redact

import "testing"

func TestJSONRedactsPassphrases(t *testing.T) {
	in := map[string]any{
		"id": "wifi-1",
		"presharedKeys": []any{
			map[string]any{"vlanId": 20.0, "passphrase": "super-secret"},
		},
		"name": "Office",
	}
	out, ok := JSON(in).(map[string]any)
	if !ok {
		t.Fatalf("%T", JSON(in))
	}
	if out["name"] != "Office" {
		t.Fatalf("name=%v", out["name"])
	}
	keys := out["presharedKeys"].([]any)
	first := keys[0].(map[string]any)
	if first["passphrase"] != placeholder {
		t.Fatalf("passphrase=%v", first["passphrase"])
	}
	if first["vlanId"] != 20.0 {
		t.Fatalf("vlanId=%v", first["vlanId"])
	}
}

func TestJSONLeavesNonSecrets(t *testing.T) {
	in := map[string]any{"enabled": true, "vlanId": 24.0}
	out := JSON(in).(map[string]any)
	if out["enabled"] != true || out["vlanId"] != 24.0 {
		t.Fatalf("%v", out)
	}
}
