package credstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetGetDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	s := New(path)
	if _, err := s.Get("home"); !os.IsNotExist(err) {
		t.Fatalf("expected not exist, got %v", err)
	}
	if err := s.Set("home", "secret-key"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("credentials should be 0600, got %v", info.Mode())
	}
	got, err := s.Get("home")
	if err != nil || got != "secret-key" {
		t.Fatalf("%v %q", err, got)
	}
	if err := s.Set("office", "other"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("home"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("home"); !os.IsNotExist(err) {
		t.Fatalf("deleted key still present: %v", err)
	}
	got, err = s.Get("office")
	if err != nil || got != "other" {
		t.Fatalf("office key lost: %v %q", err, got)
	}
}
