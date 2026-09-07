// Package state holds shotqueue's runtime model: cameras, presets, groups and queueing.
// Ported from the old server's state.js + logic.js. Nothing here is persisted directly; main
// wires OnMutate to save a "latest" version (see internal/versions) after every change, and
// reloads it via LoadVersion on startup.
package state

import (
	"errors"
	"fmt"
	"sync"

	"shotqueue-backend/internal/ptz"
)

// positionTolerance: how close a live GetPosition reading must be to a preset's captured target,
// on each of pan/tilt/zoom, to count as "this preset is active". Not zero because the camera's
// own convergence and repeated reads aren't bit-exact.
const positionTolerance = 6

var GroupColors = []string{"blue", "pink", "green", "orange", "purple", "cyan", "red", "yellow"}

type LogicError struct {
	Message string
	Status  int
}

func (e *LogicError) Error() string { return e.Message }

func newErr(status int, format string, args ...any) *LogicError {
	return &LogicError{Message: fmt.Sprintf(format, args...), Status: status}
}

type Group struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	Members []int  `json:"members"`
}

type Preset struct {
	ID               int
	Name             string
	Target           ptz.Position
	Thumbnail        []byte
	ThumbnailVersion int
	WasTriggered     bool
}

type Queued struct {
	PresetID int    `json:"presetId"`
	Origin   string `json:"origin"` // "manual" | "auto"
}

type Camera struct {
	ID                 int
	Name               string
	Host               string
	Port               string
	TallySource        uint16
	Client             *ptz.Client
	Status             string // "live" | "preview" | "none"
	Triggering         bool
	TriggeringPresetID int
	CurrentPosition    *ptz.Position
	Presets            []*Preset
	Groups             []*Group
	SelectedGroupID    *int
	Queued             *Queued
	nextGroupID        int
	Regenerating       bool
	RegenDone          int
	RegenTotal         int
}

type Store struct {
	mu           sync.Mutex
	cameras      []*Camera
	nextCameraID int
	presetsByID  map[int]*Preset
	nextPreset   int
	broadcast    func()
	onMutate     func()
	regenMu      sync.Mutex
	regenToken   int
}

func New() *Store {
	return &Store{
		nextCameraID: 1,
		presetsByID:  make(map[int]*Preset),
		nextPreset:   1,
		broadcast:    func() {},
		onMutate:     func() {},
	}
}

func (s *Store) SetBroadcaster(fn func()) { s.broadcast = fn }

// SetOnMutate registers a hook called after every mutation that changes cameras, presets or
// groups (but not runtime-only state like trigger/queue/tally). Used by main to persist a
// "latest" version after each change.
func (s *Store) SetOnMutate(fn func()) { s.onMutate = fn }

func (s *Store) withLock(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn()
}

func (s *Store) findCameraLocked(id int) *Camera {
	for _, c := range s.cameras {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// FindCamera returns a camera by id, or nil.
func (s *Store) FindCamera(id int) *Camera {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findCameraLocked(id)
}

// FindByTallySource returns the camera mapped to an ATEM tally source number, or nil.
func (s *Store) FindByTallySource(source uint16) *Camera {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cameras {
		if c.TallySource == source {
			return c
		}
	}
	return nil
}

// Cameras returns a snapshot of the camera list.
func (s *Store) Cameras() []*Camera {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Camera, len(s.cameras))
	copy(out, s.cameras)
	return out
}

func (s *Store) AddCamera(name, host, port string, tallySource uint16) (*Camera, error) {
	var cam *Camera
	s.withLock(func() {
		cam = &Camera{
			ID:          s.nextCameraID,
			Name:        name,
			Host:        host,
			Port:        port,
			TallySource: tallySource,
			Client:      ptz.New(host, port),
			Status:      "none",
			nextGroupID: 1,
		}
		s.nextCameraID++
		s.cameras = append(s.cameras, cam)
	})
	s.broadcast()
	s.onMutate()
	return cam, nil
}

func (s *Store) UpdateCamera(id int, name, host, port string, tallySource uint16) error {
	var found bool
	s.withLock(func() {
		cam := s.findCameraLocked(id)
		if cam == nil {
			return
		}
		found = true
		cam.Name = name
		if cam.Host != host || cam.Port != port {
			cam.Client = ptz.New(host, port)
		}
		cam.Host = host
		cam.Port = port
		cam.TallySource = tallySource
	})
	if !found {
		return ErrNotFound
	}
	s.broadcast()
	s.onMutate()
	return nil
}

func (s *Store) RemoveCamera(id int) error {
	var found bool
	s.withLock(func() {
		for i, c := range s.cameras {
			if c.ID == id {
				found = true
				for _, p := range c.Presets {
					delete(s.presetsByID, p.ID)
				}
				s.cameras = append(s.cameras[:i], s.cameras[i+1:]...)
				return
			}
		}
	})
	if !found {
		return ErrNotFound
	}
	s.broadcast()
	s.onMutate()
	return nil
}

// ActivePresetID reports which preset (if any) the camera's last-known position matches.
func (s *Store) ActivePresetID(cam *Camera) *int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return activePresetIDLocked(cam)
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func closeEnough(a, b ptz.Position) bool {
	return abs(a.Pan-b.Pan) <= positionTolerance && abs(a.Tilt-b.Tilt) <= positionTolerance && abs(a.Zoom-b.Zoom) <= positionTolerance
}

func activePresetIDLocked(cam *Camera) *int {
	if cam.CurrentPosition == nil {
		return nil
	}
	for _, p := range cam.Presets {
		if closeEnough(*cam.CurrentPosition, p.Target) {
			id := p.ID
			return &id
		}
	}
	return nil
}

// ClientFor returns the PTZ client for a camera, safe to use concurrently with camera edits.
func (s *Store) ClientFor(cameraID int) (*ptz.Client, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return nil, false
	}
	return cam.Client, true
}

func (s *Store) PresetThumbnail(presetID int) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.presetsByID[presetID]
	if !ok {
		return nil, false
	}
	return p.Thumbnail, true
}

var ErrNotFound = errors.New("not found")
