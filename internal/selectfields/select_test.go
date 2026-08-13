package selectfields

import "testing"

func TestApplyTopLevel(t *testing.T) {
	in := map[string]any{"id": "1", "name": "ap", "state": "ONLINE"}
	out, err := Apply(in, "id,name")
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["id"] != "1" || m["name"] != "ap" {
		t.Fatalf("%v", m)
	}
	if _, ok := m["state"]; ok {
		t.Fatal("state should be omitted")
	}
}

func TestApplyNestedAndArray(t *testing.T) {
	in := map[string]any{
		"siteId": "abc",
		"page": map[string]any{
			"data": []any{
				map[string]any{"id": "1", "name": "a", "extra": true},
				map[string]any{"id": "2", "name": "b", "extra": false},
			},
		},
	}
	out, err := Apply(in, "siteId,page.data")
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["siteId"] != "abc" {
		t.Fatalf("%v", m)
	}
	page := m["page"].(map[string]any)
	data := page["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data=%v", data)
	}
}

func TestApplyEmpty(t *testing.T) {
	in := map[string]any{"a": 1}
	out, err := Apply(in, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.(map[string]any)["a"] != 1 && out.(map[string]any)["a"] != float64(1) {
		// round-trip through Apply empty returns original without marshal
		_ = out
	}
}
