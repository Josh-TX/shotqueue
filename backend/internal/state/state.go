// Package state holds shotqueue's runtime model: cameras, presets, groups and queueing.
// Ported from the old server's state.js + logic.js. Only the camera roster is persisted (via
// internal/config); presets/groups live in memory only, same as before.
package state

import (
	"errors"
	"fmt"
	"sync"

	"shotqueue-backend/internal/config"
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
	mu          sync.Mutex
	cfg         *config.Store
	cameras     []*Camera
	presetsByID map[int]*Preset
	nextPreset  int
	broadcast   func()
	regenMu     sync.Mutex
	regenToken  int
}

func New(cfg *config.Store) *Store {
	s := &Store{cfg: cfg, presetsByID: make(map[int]*Preset), nextPreset: 1, broadcast: func() {}}
	for _, c := range cfg.Get().Cameras {
		s.cameras = append(s.cameras, &Camera{
			ID:          c.ID,
			Name:        c.Name,
			Host:        c.Host,
			Port:        c.Port,
			TallySource: c.TallySource,
			Client:      ptz.New(c.Host, c.Port),
			Status:      "none",
			nextGroupID: 1,
		})
	}
	return s
}

func (s *Store) SetBroadcaster(fn func()) { s.broadcast = fn }

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
	cfgCam, err := s.cfg.AddCamera(name, host, port, tallySource)
	if err != nil {
		return nil, err
	}
	cam := &Camera{
		ID:          cfgCam.ID,
		Name:        cfgCam.Name,
		Host:        cfgCam.Host,
		Port:        cfgCam.Port,
		TallySource: cfgCam.TallySource,
		Client:      ptz.New(cfgCam.Host, cfgCam.Port),
		Status:      "none",
		nextGroupID: 1,
	}
	s.withLock(func() { s.cameras = append(s.cameras, cam) })
	s.broadcast()
	return cam, nil
}

func (s *Store) UpdateCamera(id int, name, host, port string, tallySource uint16) error {
	if err := s.cfg.UpdateCamera(id, name, host, port, tallySource); err != nil {
		return err
	}
	s.withLock(func() {
		cam := s.findCameraLocked(id)
		if cam == nil {
			return
		}
		cam.Name = name
		if cam.Host != host || cam.Port != port {
			cam.Client = ptz.New(host, port)
		}
		cam.Host = host
		cam.Port = port
		cam.TallySource = tallySource
	})
	s.broadcast()
	return nil
}

func (s *Store) RemoveCamera(id int) error {
	if err := s.cfg.RemoveCamera(id); err != nil {
		return err
	}
	s.withLock(func() {
		for i, c := range s.cameras {
			if c.ID == id {
				for _, p := range c.Presets {
					delete(s.presetsByID, p.ID)
				}
				s.cameras = append(s.cameras[:i], s.cameras[i+1:]...)
				return
			}
		}
	})
	s.broadcast()
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
