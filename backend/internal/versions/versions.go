// Package versions loads and saves shotqueue's saved "versions" — snapshots of the full camera
// roster plus each camera's presets and groups — to a JSON file in the user's config directory,
// same pattern as internal/config. Camera/preset/group IDs are never stored here: on load, the
// state package assigns fresh IDs, so group members reference presets by index within their
// camera's Presets slice instead.
package versions

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"shotqueue-backend/internal/ptz"
)

// AutosaveKeepCount is how many autosaved versions are kept before the oldest is evicted. Small
// for now; expected to grow to something like 30 once this has proven out.
const AutosaveKeepCount = 3

type VersionPreset struct {
	Name   string       `json:"name"`
	Target ptz.Position `json:"target"`
}

type VersionGroupMember struct {
	PresetIndex int `json:"presetIndex"`
	Weight      int `json:"weight"`
}

type VersionGroup struct {
	Name    string               `json:"name"`
	Color   string               `json:"color"`
	Members []VersionGroupMember `json:"members"`
}

type VersionCamera struct {
	Name        string          `json:"name"`
	Host        string          `json:"host"`
	Port        string          `json:"port"`
	TallySource uint16          `json:"tallySource"`
	Presets     []VersionPreset `json:"presets"`
	Groups      []VersionGroup  `json:"groups"`
}

type Version struct {
	ID        int             `json:"id"`
	Type      string          `json:"type"` // "named" | "autosave"
	Name      string          `json:"name"` // empty for autosave
	Timestamp int64           `json:"timestamp"`
	Cameras   []VersionCamera `json:"cameras"`
}

type Store struct {
	mu                  sync.Mutex
	path                string
	versions            []Version
	nextID              int
	lastAutosavePayload []byte
}

func dirPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "shotqueue"), nil
}

type fileFormat struct {
	Versions []Version `json:"versions"`
	NextID   int       `json:"nextId"`
}

// Load reads versions.json from the user config directory, creating an empty default if it
// doesn't exist yet.
func Load() (*Store, error) {
	dir, err := dirPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "versions.json")

	s := &Store{path: path, nextID: 1}

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
	s.versions = f.Versions
	s.nextID = f.NextID
	if s.nextID == 0 {
		s.nextID = 1
	}
	for _, v := range s.versions {
		if v.Type == "autosave" {
			if payload, err := json.Marshal(v.Cameras); err == nil {
				s.lastAutosavePayload = payload
			}
		}
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(fileFormat{Versions: s.versions, NextID: s.nextID}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// List returns every version (named and autosave), in no particular order — callers sort as
// needed for display.
func (s *Store) List() []Version {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Version, len(s.versions))
	copy(out, s.versions)
	return out
}

func (s *Store) Get(id int) (Version, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.versions {
		if v.ID == id {
			return v, true
		}
	}
	return Version{}, false
}

// SaveNamed creates a new named version, or overwrites the existing one whose name matches
// case-insensitively, refreshing its timestamp either way.
func (s *Store) SaveNamed(name string, snapshot []VersionCamera) (Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lower := strings.ToLower(name)
	for i := range s.versions {
		if s.versions[i].Type == "named" && strings.ToLower(s.versions[i].Name) == lower {
			s.versions[i].Cameras = snapshot
			s.versions[i].Timestamp = time.Now().UnixMilli()
			if err := s.saveLocked(); err != nil {
				return Version{}, err
			}
			return s.versions[i], nil
		}
	}
	v := Version{ID: s.nextID, Type: "named", Name: name, Timestamp: time.Now().UnixMilli(), Cameras: snapshot}
	s.nextID++
	s.versions = append(s.versions, v)
	if err := s.saveLocked(); err != nil {
		return Version{}, err
	}
	return v, nil
}

// DeleteNamed removes a named version. Autosaved versions can only be evicted automatically, not
// deleted directly.
func (s *Store) DeleteNamed(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.versions {
		if v.ID == id {
			if v.Type != "named" {
				return errors.New("cannot delete an autosaved version")
			}
			s.versions = append(s.versions[:i], s.versions[i+1:]...)
			return s.saveLocked()
		}
	}
	return errors.New("version not found")
}

// MaybeAutosave saves a new autosave version if the snapshot differs from the last autosave
// (ignoring name/timestamp/id, which aren't part of the snapshot anyway), then evicts the oldest
// autosave(s) beyond AutosaveKeepCount.
func (s *Store) MaybeAutosave(snapshot []VersionCamera) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastAutosavePayload != nil && string(payload) == string(s.lastAutosavePayload) {
		return
	}
	v := Version{ID: s.nextID, Type: "autosave", Timestamp: time.Now().UnixMilli(), Cameras: snapshot}
	s.nextID++
	s.versions = append(s.versions, v)
	s.lastAutosavePayload = payload
	s.evictOldAutosavesLocked()
	s.saveLocked()
}

// SetAutosaveBaseline records snapshot as the new "last autosave" content for dirty-checking,
// without creating a version entry. Used after loading a version, so the autosave timer doesn't
// immediately fire again for content that was just loaded rather than changed.
func (s *Store) SetAutosaveBaseline(snapshot []VersionCamera) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.lastAutosavePayload = payload
	s.mu.Unlock()
}

// evictOldAutosavesLocked keeps only the AutosaveKeepCount most recent autosaves. Autosaves are
// always appended in chronological order, so walking from the end keeps the newest ones.
func (s *Store) evictOldAutosavesLocked() {
	kept := 0
	for i := len(s.versions) - 1; i >= 0; i-- {
		if s.versions[i].Type != "autosave" {
			continue
		}
		kept++
		if kept > AutosaveKeepCount {
			s.versions = append(s.versions[:i], s.versions[i+1:]...)
		}
	}
}
