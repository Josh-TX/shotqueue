// Package settings loads and saves shotqueue's settings (currently just the ATEM address) to a
// JSON file in the user's config directory. The camera roster, presets and groups are persisted
// separately via internal/config (the "latest" config), not here.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Atem struct {
	Host string `json:"host"`
}

type Settings struct {
	Atem Atem `json:"atem"`
}

type Store struct {
	mu   sync.Mutex
	path string
	cfg  Settings
}

func dirPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "shotqueue"), nil
}

// Load reads settings.json from the user config directory, creating an empty default if it doesn't
// exist yet.
func Load() (*Store, error) {
	dir, err := dirPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "settings.json")

	s := &Store{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, s.saveLocked()
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &s.cfg); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *Store) SetAtem(atem Atem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Atem = atem
	return s.saveLocked()
}
