package credstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store persists API keys keyed by profile name in a 0600 JSON file.
// OS keyring support can be added later without changing callers.
type Store struct {
	path string
	mu   sync.Mutex
}

type fileData struct {
	Keys map[string]string `json:"keys"`
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "unicli", "credentials.json"), nil
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Get(profile string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return "", err
	}
	k, ok := data.Keys[profile]
	if !ok || k == "" {
		return "", os.ErrNotExist
	}
	return k, nil
}

func (s *Store) Set(profile, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if data.Keys == nil {
		data.Keys = map[string]string{}
	}
	data.Keys[profile] = key
	return s.save(data)
}

func (s *Store) Delete(profile string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	delete(data.Keys, profile)
	return s.save(data)
}

func (s *Store) load() (fileData, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fileData{Keys: map[string]string{}}, err
	}
	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fileData{}, err
	}
	if data.Keys == nil {
		data.Keys = map[string]string{}
	}
	return data, nil
}

func (s *Store) save(data fileData) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
