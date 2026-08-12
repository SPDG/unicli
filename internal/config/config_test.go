package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"192.168.1.1":          "https://192.168.1.1",
		"https://unifi.local/": "https://unifi.local",
		"http://10.0.0.1":      "http://10.0.0.1",
	}
	for in, want := range cases {
		if got := NormalizeHost(in); got != want {
			t.Fatalf("NormalizeHost(%q)=%q want %q", in, got, want)
		}
	}
}

func TestResolveEnvWinsOverProfile(t *testing.T) {
	t.Setenv(EnvHost, "https://env.example")
	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvInsecure, "1")
	t.Setenv(EnvProfile, "")

	cfg := &File{
		Current: "home",
		Profiles: map[string]Profile{
			"home": {Host: "https://home.example", Insecure: false},
		},
	}
	got, err := Resolve(cfg, ResolveOptions{}, func(string) (string, error) {
		return "profile-key", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "https://env.example" || got.APIKey != "env-key" || !got.Insecure {
		t.Fatalf("unexpected resolve: %+v", got)
	}
}

func TestResolveFallsBackToCurrentProfile(t *testing.T) {
	t.Setenv(EnvHost, "")
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvProfile, "")
	t.Setenv(EnvInsecure, "")

	cfg := &File{
		Current: "lab",
		Profiles: map[string]Profile{
			"lab": {Host: "https://lab.example", Insecure: true, Site: "default-site"},
		},
	}
	got, err := Resolve(cfg, ResolveOptions{}, func(name string) (string, error) {
		if name != "lab" {
			t.Fatalf("unexpected profile %q", name)
		}
		return "lab-key", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile != "lab" || got.Host != "https://lab.example" || got.APIKey != "lab-key" || !got.Insecure || got.Site != "default-site" {
		t.Fatalf("unexpected resolve: %+v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	f := &File{
		Current: "a",
		Profiles: map[string]Profile{
			"a": {Host: "https://a.example", Insecure: true},
			"b": {Host: "https://b.example"},
		},
	}
	if err := Save(path, f); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("config should not be group/world readable: %v", info.Mode())
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Current != "a" || loaded.Profiles["b"].Host != "https://b.example" {
		t.Fatalf("bad load: %+v", loaded)
	}
}
