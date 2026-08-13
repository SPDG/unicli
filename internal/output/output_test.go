package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestResolveFormat(t *testing.T) {
	if ResolveFormat(true, false) != FormatJSON {
		t.Fatal("json")
	}
	if ResolveFormat(false, true) != FormatPlain {
		t.Fatal("plain")
	}
	if ResolveFormat(false, false) != FormatAuto {
		t.Fatal("auto")
	}
}

func TestWantJSONForced(t *testing.T) {
	if !WantJSON(FormatJSON, nil) {
		t.Fatal("json")
	}
	if WantJSON(FormatPlain, nil) {
		t.Fatal("plain")
	}
}

func TestWriteJSONAndError(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil || m["a"] != "b" {
		t.Fatalf("%v %s", err, buf.String())
	}
	buf.Reset()
	if err := WriteError(&buf, "AUTH_REQUIRED", "no key", "unicli auth login"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "AUTH_REQUIRED") {
		t.Fatal(buf.String())
	}
}
