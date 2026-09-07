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
	"sort"
	"strings"
	"sync"
	"time"

	"shotqueue-backend/internal/ptz"
)

// AutosaveKeepCount is how many autosaved versions are kept before the oldest is evicted. Small
// for now; expected to grow to something like 30 once this has proven out.
const AutosaveKeepCount = 5

// autosaveCheckpointGap is how far apart (by Timestamp) the two most recent autosaves must be
// before Autosave splits off a new entry instead of overwriting the latest one in place.
const autosaveCheckpointGap = 20 * time.Minute

type VersionPreset struct {
	Name   string       `json:"name"`
	Target ptz.Position `json:"target"`
}

type VersionGroup struct {
	Name    string `json:"name"`
	Color   string `json:"color"`
	Members []int  `json:"members"` // preset indices within the camera's Presets slice
}

type VersionCamera struct {
	Name        string          `json:"name"`
	Host        string          `json:"host"`
	Port        string          `json:"port"`
	TallySource uint16          `json:"tallySource"`
	ColumnCount int             `json:"columnCount"`
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
	mu       sync.Mutex
	path     string
	versions []Version
	nextID   int
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

// LatestAutosave returns the most recently timestamped autosave version, if any exist yet.
// This is what a fresh boot reloads to resume exactly where things left off.
func (s *Store) LatestAutosave() (Version, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.latestAutosaveIdxLocked()
	if idx == -1 {
		return Version{}, false
	}
	return s.versions[idx], true
}

// latestAutosaveIdxLocked returns the index of the autosave with the greatest Timestamp, or -1.
func (s *Store) latestAutosaveIdxLocked() int {
	best := -1
	for i, v := range s.versions {
		if v.Type != "autosave" {
			continue
		}
		if best == -1 || v.Timestamp > s.versions[best].Timestamp {
			best = i
		}
	}
	return best
}

// Autosave records snapshot as the current live state, called after every mutation that changes
// cameras, presets or groups (including loading another version, named or autosaved, which
// immediately becomes the new latest autosave). Rather than rewriting a single dedicated "latest"
// slot forever, it keeps a trail of checkpoints: if the current latest autosave is more than
// autosaveCheckpointGap newer than the one before it, that latest is left alone as history and
// snapshot becomes a brand new entry; otherwise snapshot simply overwrites the latest in place
// (bumping its Timestamp to now), so a burst of rapid edits collapses into one checkpoint. Oldest
// autosaves beyond AutosaveKeepCount are evicted afterward.
func (s *Store) Autosave(snapshot []VersionCamera) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	latestIdx := s.latestAutosaveIdxLocked()

	var secondIdx int = -1
	if latestIdx != -1 {
		for i, v := range s.versions {
			if i == latestIdx || v.Type != "autosave" {
				continue
			}
			if secondIdx == -1 || v.Timestamp > s.versions[secondIdx].Timestamp {
				secondIdx = i
			}
		}
	}

	checkpoint := latestIdx == -1 || secondIdx == -1 ||
		s.versions[latestIdx].Timestamp-s.versions[secondIdx].Timestamp > autosaveCheckpointGap.Milliseconds()

	if !checkpoint {
		s.versions[latestIdx].Cameras = snapshot
		s.versions[latestIdx].Timestamp = now
	} else {
		v := Version{ID: s.nextID, Type: "autosave", Timestamp: now, Cameras: snapshot}
		s.nextID++
		s.versions = append(s.versions, v)
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
	for i, v := range s.versions {
		if v.Type == "autosave" {
			autosaves = append(autosaves, indexed{i, v.Timestamp})
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
	out := s.versions[:0]
	for i, v := range s.versions {
		if !evict[i] {
			out = append(out, v)
		}
	}
	s.versions = out
}
