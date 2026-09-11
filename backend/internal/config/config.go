// Package config loads and saves shotqueue's saved "configs" — snapshots of the full camera
// roster plus each camera's presets and groups — to a JSON file in the user's config directory,
// same pattern as internal/settings. Camera/preset/group IDs are never stored here: on load, the
// state package assigns fresh IDs, so group members reference presets by index within their
// camera's Presets slice instead.
package config

import (
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"shotqueue-backend/internal/ptz"
)

const idChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// genID returns a random 9-character alphanumeric ID. Collisions are astronomically unlikely
// (62^9 possibilities) and are not checked for.
func genID() string {
	b := make([]byte, 9)
	for i := range b {
		b[i] = idChars[rand.Intn(len(idChars))]
	}
	return string(b)
}

// AutosaveKeepCount is how many autosaved configs are kept before the oldest is evicted.
const AutosaveKeepCount = 50

// autosaveCheckpointGap is how far apart (by Timestamp) the two most recent autosaves must be
// before Autosave splits off a new entry instead of overwriting the latest one in place.
const autosaveCheckpointGap = 10 * time.Minute

type ConfigPreset struct {
	Name   string       `json:"name"`
	Target ptz.Position `json:"target"`
}

type ConfigGroup struct {
	Name       string `json:"name"`
	Members    []int  `json:"members"` // preset indices within the camera's Presets slice
	IsSequence bool   `json:"isSequence"`
}

type ConfigCamera struct {
	CameraNum   int            `json:"cameraNum"`
	ColumnCount int            `json:"columnCount"`
	Presets     []ConfigPreset `json:"presets"`
	Groups      []ConfigGroup  `json:"groups"`
}

type Config struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"` // "named" | "autosave"
	Name      string         `json:"name"` // empty for autosave
	Timestamp int64          `json:"timestamp"`
	Cameras   []ConfigCamera `json:"cameras"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	configs []Config
}

func dirPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "shotqueue"), nil
}

type fileFormat struct {
	Configs []Config `json:"configs"`
}

// Load reads configs.json from the user config directory, creating an empty default if it
// doesn't exist yet.
func Load() (*Store, error) {
	dir, err := dirPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "configs.json")

	s := &Store{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, s.saveLocked()
		}
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	s.configs = f.Configs
	return s, nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(fileFormat{Configs: s.configs}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// List returns every config (named and autosave), in no particular order — callers sort as
// needed for display.
func (s *Store) List() []Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Config, len(s.configs))
	copy(out, s.configs)
	return out
}

func (s *Store) Get(id string) (Config, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.configs {
		if c.ID == id {
			return c, true
		}
	}
	return Config{}, false
}

// SaveNamed creates a new named config, or overwrites the existing one whose name matches
// case-insensitively, refreshing its timestamp either way.
func (s *Store) SaveNamed(name string, snapshot []ConfigCamera) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lower := strings.ToLower(name)
	for i := range s.configs {
		if s.configs[i].Type == "named" && strings.ToLower(s.configs[i].Name) == lower {
			s.configs[i].Cameras = snapshot
			s.configs[i].Timestamp = time.Now().UnixMilli()
			if err := s.saveLocked(); err != nil {
				return Config{}, err
			}
			return s.configs[i], nil
		}
	}
	c := Config{ID: genID(), Type: "named", Name: name, Timestamp: time.Now().UnixMilli(), Cameras: snapshot}
	s.configs = append(s.configs, c)
	if err := s.saveLocked(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// DeleteNamed removes a named config. Autosaved configs can only be evicted automatically, not
// deleted directly.
func (s *Store) DeleteNamed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.configs {
		if c.ID == id {
			if c.Type != "named" {
				return errors.New("cannot delete an autosaved config")
			}
			s.configs = append(s.configs[:i], s.configs[i+1:]...)
			return s.saveLocked()
		}
	}
	return errors.New("config not found")
}

// LatestAutosave returns the most recently timestamped autosave config, if any exist yet.
// This is what a fresh boot reloads to resume exactly where things left off.
func (s *Store) LatestAutosave() (Config, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.latestAutosaveIdxLocked()
	if idx == -1 {
		return Config{}, false
	}
	return s.configs[idx], true
}

// latestAutosaveIdxLocked returns the index of the autosave with the greatest Timestamp, or -1.
func (s *Store) latestAutosaveIdxLocked() int {
	best := -1
	for i, c := range s.configs {
		if c.Type != "autosave" {
			continue
		}
		if best == -1 || c.Timestamp > s.configs[best].Timestamp {
			best = i
		}
	}
	return best
}

// Autosave records snapshot as the current live state, called after every mutation that changes
// cameras, presets or groups (including loading another config, named or autosaved, which
// immediately becomes the new latest autosave). Rather than rewriting a single dedicated "latest"
// slot forever, it keeps a trail of checkpoints: if the current latest autosave is more than
// autosaveCheckpointGap newer than the one before it, that latest is left alone as history and
// snapshot becomes a brand new entry; otherwise snapshot simply overwrites the latest in place
// (bumping its Timestamp to now), so a burst of rapid edits collapses into one checkpoint. Oldest
// autosaves beyond AutosaveKeepCount are evicted afterward.
func (s *Store) Autosave(snapshot []ConfigCamera) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	latestIdx := s.latestAutosaveIdxLocked()

	var secondIdx int = -1
	if latestIdx != -1 {
		for i, c := range s.configs {
			if i == latestIdx || c.Type != "autosave" {
				continue
			}
			if secondIdx == -1 || c.Timestamp > s.configs[secondIdx].Timestamp {
				secondIdx = i
			}
		}
	}

	checkpoint := latestIdx == -1 || secondIdx == -1 ||
		s.configs[latestIdx].Timestamp-s.configs[secondIdx].Timestamp > autosaveCheckpointGap.Milliseconds()

	if !checkpoint {
		s.configs[latestIdx].Cameras = snapshot
		s.configs[latestIdx].Timestamp = now
	} else {
		c := Config{ID: genID(), Type: "autosave", Timestamp: now, Cameras: snapshot}
		s.configs = append(s.configs, c)
		s.evictOldAutosavesLocked()
	}
	return s.saveLocked()
}

// evictOldAutosavesLocked keeps only the AutosaveKeepCount most recent autosaves (by Timestamp).
func (s *Store) evictOldAutosavesLocked() {
	type indexed struct {
		idx int
		ts  int64
	}
	var autosaves []indexed
	for i, c := range s.configs {
		if c.Type == "autosave" {
			autosaves = append(autosaves, indexed{i, c.Timestamp})
		}
	}
	if len(autosaves) <= AutosaveKeepCount {
		return
	}
	sort.Slice(autosaves, func(a, b int) bool { return autosaves[a].ts > autosaves[b].ts })
	evict := map[int]bool{}
	for _, a := range autosaves[AutosaveKeepCount:] {
		evict[a.idx] = true
	}
	out := s.configs[:0]
	for i, c := range s.configs {
		if !evict[i] {
			out = append(out, c)
		}
	}
	s.configs = out
}
