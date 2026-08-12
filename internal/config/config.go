package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	EnvHost     = "UNIFI_HOST"
	EnvAPIKey   = "UNIFI_API_KEY"
	EnvProfile  = "UNIFI_PROFILE"
	EnvInsecure = "UNIFI_INSECURE"
	EnvSite     = "UNIFI_SITE"
)

// File is the on-disk multi-gateway configuration.
type File struct {
	Current  string             `yaml:"current"`
	Profiles map[string]Profile `yaml:"profiles"`
}

// Profile describes one UniFi console / gateway.
type Profile struct {
	Host     string `yaml:"host"`
	Insecure bool   `yaml:"insecure"`
	Site     string `yaml:"site,omitempty"`
}

// Resolved is the effective connection after flag/env/config merge.
type Resolved struct {
	Profile  string
	Host     string
	APIKey   string
	Insecure bool
	Site     string
	Source   string // "env", "profile:<name>", etc.
}

type ResolveOptions struct {
	Profile  string
	Host     string
	Insecure *bool
	Site     string
	// APIKey from env only; never from CLI argv in callers.
	APIKey string
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "unicli", "config.yaml"), nil
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{Profiles: map[string]Profile{}}, nil
		}
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	return &f, nil
}

func Save(path string, f *File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ParseInsecureEnv(v string) (bool, bool) {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return false, false
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b, true
	}
	return false, false
}

// Resolve merges CLI options, environment, and config profiles.
// Precedence: explicit opts (flags) > env > selected profile > error.
func Resolve(cfg *File, opts ResolveOptions, keyForProfile func(profile string) (string, error)) (*Resolved, error) {
	if cfg == nil {
		cfg = &File{Profiles: map[string]Profile{}}
	}

	profileName := strings.TrimSpace(opts.Profile)
	if profileName == "" {
		profileName = strings.TrimSpace(os.Getenv(EnvProfile))
	}
	if profileName == "" {
		profileName = strings.TrimSpace(cfg.Current)
	}

	var (
		host     = strings.TrimSpace(opts.Host)
		apiKey   = strings.TrimSpace(opts.APIKey)
		site     = strings.TrimSpace(opts.Site)
		insecure bool
		insSet   bool
		source   string
	)

	if opts.Insecure != nil {
		insecure = *opts.Insecure
		insSet = true
	}

	if host == "" {
		host = strings.TrimSpace(os.Getenv(EnvHost))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv(EnvAPIKey))
	}
	if site == "" {
		site = strings.TrimSpace(os.Getenv(EnvSite))
	}
	if !insSet {
		if v, ok := ParseInsecureEnv(os.Getenv(EnvInsecure)); ok {
			insecure = v
			insSet = true
		}
	}

	envProvided := host != "" || apiKey != "" || os.Getenv(EnvHost) != "" || os.Getenv(EnvAPIKey) != ""

	var prof Profile
	var haveProf bool
	if profileName != "" {
		if p, ok := cfg.Profiles[profileName]; ok {
			prof = p
			haveProf = true
		} else if !envFullySpecified(host, apiKey) {
			return nil, fmt.Errorf("%w: profile %q not found (unicli profile list)", errConfig, profileName)
		}
	}

	if haveProf {
		if host == "" {
			host = strings.TrimSpace(prof.Host)
		}
		if site == "" {
			site = strings.TrimSpace(prof.Site)
		}
		if !insSet {
			insecure = prof.Insecure
			insSet = true
		}
		if apiKey == "" && keyForProfile != nil {
			k, err := keyForProfile(profileName)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			apiKey = strings.TrimSpace(k)
		}
		if source == "" {
			source = "profile:" + profileName
		}
	}

	if envProvided && (os.Getenv(EnvHost) != "" || os.Getenv(EnvAPIKey) != "") {
		source = "env"
		if profileName != "" && haveProf {
			source = "env+profile:" + profileName
		}
	}

	if host == "" {
		return nil, fmt.Errorf("%w: host not set (export %s or add a profile)", errConfig, EnvHost)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("%w: API key not set (export %s or: unicli auth login)", errConfig, EnvAPIKey)
	}

	return &Resolved{
		Profile:  profileName,
		Host:     NormalizeHost(host),
		APIKey:   apiKey,
		Insecure: insecure,
		Site:     site,
		Source:   source,
	}, nil
}

func envFullySpecified(host, key string) bool {
	return host != "" && key != ""
}

var errConfig = errors.New("config error")

func IsConfigError(err error) bool {
	return errors.Is(err, errConfig)
}

// NormalizeHost accepts https://host, host, or host:port and returns scheme://host[:port].
func NormalizeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimRight(host, "/")
	if host == "" {
		return host
	}
	if !strings.Contains(host, "://") {
		return "https://" + host
	}
	return host
}
