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

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, []string{"NAME", "IP"}, [][]string{
		{"pi", "192.168.5.221"},
		{"", "192.168.5.1"},
	}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "pi") || !strings.Contains(got, "-") {
		t.Fatal(got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines=%d %q", len(lines), got)
	}
}

func TestPageFooter(t *testing.T) {
	if got := PageFooter(0, 23, 23); got != "23 items" {
		t.Fatal(got)
	}
	if got := PageFooter(0, 25, 87); got != "showing 1-25 of 87  (more: --offset 25)" {
		t.Fatal(got)
	}
	if got := PageFooter(25, 25, 87); got != "showing 26-50 of 87  (more: --offset 50)" {
		t.Fatal(got)
	}
	if got := PageFooter(75, 12, 87); got != "showing 76-87 of 87" {
		t.Fatal(got)
	}
	if got := PageFooter(0, 0, 0); got != "0 items" {
		t.Fatal(got)
	}
}
