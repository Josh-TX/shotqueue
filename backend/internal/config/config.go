// Package config loads and saves shotqueue's settings (ATEM address, camera roster) to a JSON
// file in the user's config directory. This is the only thing that survives a restart — presets,
// groups and metrics are runtime state (see internal/state) and are not persisted here.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Atem struct {
	Host string `json:"host"`
}

type Camera struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        string `json:"port"`
	TallySource uint16 `json:"tallySource"`
}

type Config struct {
	Atem         Atem     `json:"atem"`
	Cameras      []Camera `json:"cameras"`
	NextCameraID int      `json:"nextCameraId"`
}

type Store struct {
	mu   sync.Mutex
	path string
	cfg  Config
}

func dirPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "shotqueue"), nil
}

// Load reads config.json from the user config directory, creating an empty default if it doesn't
// exist yet.
func Load() (*Store, error) {
	dir, err := dirPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")

	s := &Store{path: path, cfg: Config{NextCameraID: 1}}

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

func (s *Store) Get() Config {
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

func (s *Store) AddCamera(name, host, port string, tallySource uint16) (Camera, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := Camera{ID: s.cfg.NextCameraID, Name: name, Host: host, Port: port, TallySource: tallySource}
	s.cfg.NextCameraID++
	s.cfg.Cameras = append(s.cfg.Cameras, cam)
	return cam, s.saveLocked()
}

func (s *Store) UpdateCamera(id int, name, host, port string, tallySource uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cfg.Cameras {
		if s.cfg.Cameras[i].ID == id {
			s.cfg.Cameras[i].Name = name
			s.cfg.Cameras[i].Host = host
			s.cfg.Cameras[i].Port = port
			s.cfg.Cameras[i].TallySource = tallySource
			return s.saveLocked()
		}
	}
	return os.ErrNotExist
}

func (s *Store) RemoveCamera(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.cfg.Cameras[:0]
	for _, c := range s.cfg.Cameras {
		if c.ID != id {
			out = append(out, c)
		}
	}
	s.cfg.Cameras = out
	return s.saveLocked()
}
