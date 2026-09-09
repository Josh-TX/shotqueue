// Package state holds shotqueue's runtime model: cameras, presets, groups and queueing.
// Ported from the old server's state.js + logic.js. Nothing here is persisted directly; main
// wires OnMutate to save a "latest" config (see internal/config) after every change, and
// reloads it via LoadConfig on startup.
package state

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"shotqueue-backend/internal/atem"
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

// idlePollInterval is how often Start's poll loop re-reads every camera's live position, to catch
// moves made by anything other than TriggerPreset (an external controller, a physical joystick,
// etc). Only runs while at least one websocket client is connected.
const idlePollInterval = 1 * time.Second

// positionTolerance: how close a live GetPosition reading must be to a preset's captured target,
// on each of pan/tilt/zoom, to count as "this preset is active". Not zero because the camera's
// own convergence and repeated reads aren't bit-exact.
const positionTolerance = 6

type LogicError struct {
	Message string
	Status  int
}

func (e *LogicError) Error() string { return e.Message }

func newErr(status int, format string, args ...any) *LogicError {
	return &LogicError{Message: fmt.Sprintf(format, args...), Status: status}
}

type Group struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Members    []string `json:"members"`
	IsSequence bool     `json:"isSequence"`
}

// MaxGroupCount is the largest group count selectable per camera (see SetGroupCount).
const MaxGroupCount = 4

// DefaultGroupCount is how many groups a newly added camera starts with.
const DefaultGroupCount = 2

type Preset struct {
	ID               string
	Name             string
	Target           ptz.Position
	Thumbnail        []byte
	ThumbnailVersion int
	WasTaken         bool
}

type Queued struct {
	PresetID string `json:"presetId"`
	Origin   string `json:"origin"` // "manual" | "auto"
}

type Camera struct {
	ID                      string
	Name                    string
	Host                    string
	Port                    string
	TallySource             uint16
	Client                  *ptz.Client
	Status                  string // "live" | "preview" | "none"
	TriggeringPresetID      *string
	TriggerGen              int
	CurrentPosition         *ptz.Position
	ActivePresetID          *string
	Presets                 []*Preset
	Groups                  []*Group
	ColumnCount             int
	SelectedGroupID         *int
	Queued                  *Queued
	nextGroupID             int
	Generating              bool
}

type Store struct {
	mu            sync.Mutex
	cameras       []*Camera
	presetsByID   map[string]*Preset
	broadcast     func()
	onMutate      func()
	genMu         sync.Mutex
	genToken      int
	pollMu        sync.Mutex
	pollStop      chan struct{}
	atemConnected bool
	lastTally     atem.TallyState
}

func New() *Store {
	return &Store{
		presetsByID: make(map[string]*Preset),
		broadcast:   func() {},
		onMutate:    func() {},
	}
}

func (s *Store) SetBroadcaster(fn func()) { s.broadcast = fn }

// SetAtemConnected records whether the ATEM listener currently has a live connection and
// broadcasts the change to connected clients.
func (s *Store) SetAtemConnected(v bool) {
	s.withLock(func() { s.atemConnected = v })
	s.broadcast()
}

func (s *Store) AtemConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.atemConnected
}

// Start begins polling every camera's live position on idlePollInterval, to detect moves made
// outside of TriggerPreset. The caller (api.Server) invokes this when the first websocket client
// connects, and Stop when the last one disconnects, so idle cameras aren't polled for nothing.
func (s *Store) Start() {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	if s.pollStop != nil {
		return
	}
	stop := make(chan struct{})
	s.pollStop = stop
	go s.pollLoop(stop)
}

func (s *Store) Stop() {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	if s.pollStop == nil {
		return
	}
	close(s.pollStop)
	s.pollStop = nil
}

func (s *Store) pollLoop(stop chan struct{}) {
	s.pollTick()
	ticker := time.NewTicker(idlePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.pollTick()
		}
	}
}

// pollTick re-reads every camera's live position and broadcasts once if any camera's
// activePresetId actually changed as a result. When a camera lands on a preset this way (e.g.
// moved by an external controller rather than TriggerPreset), it also refreshes that preset's
// thumbnail, so the thumbnail stays in sync regardless of what caused the move.
func (s *Store) pollTick() {
	changed := false
	for _, cam := range s.Cameras() {
		before := s.ActivePresetID(cam)
		s.RefreshPosition(cam.ID)
		after := s.ActivePresetID(cam)
		if !strPtrEqual(before, after) {
			changed = true
			if after != nil {
				go s.refreshPresetThumbnail(cam.ID, *after)
			}
		}
	}
	if changed {
		s.broadcast()
	}
}

func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// SetOnMutate registers a hook called after every mutation that changes cameras, presets or
// groups (but not runtime-only state like trigger/queue/tally). Used by main to persist a
// "latest" config after each change.
func (s *Store) SetOnMutate(fn func()) { s.onMutate = fn }

func (s *Store) withLock(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn()
}

func (s *Store) findCameraLocked(id string) *Camera {
	for _, c := range s.cameras {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// FindCamera returns a camera by id, or nil.
func (s *Store) FindCamera(id string) *Camera {
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
			ID:          genID(),
			Name:        name,
			Host:        host,
			Port:        port,
			TallySource: tallySource,
			Client:      ptz.New(host, port),
			Status:      "none",
			ColumnCount: 2,
			nextGroupID: 1,
		}
		for i := 0; i < DefaultGroupCount; i++ {
			cam.Groups = append(cam.Groups, &Group{ID: cam.nextGroupID, Name: fmt.Sprintf("Group %d", i+1)})
			cam.nextGroupID++
		}
		s.cameras = append(s.cameras, cam)
	})
	s.reconcileCameraTally(cam)
	s.broadcast()
	s.onMutate()
	return cam, nil
}

func (s *Store) UpdateCamera(id string, name, host, port string, tallySource uint16, columnCount int) error {
	var found bool
	var cam *Camera
	var tallySourceChanged bool
	s.withLock(func() {
		cam = s.findCameraLocked(id)
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
		tallySourceChanged = cam.TallySource != tallySource
		cam.TallySource = tallySource
		cam.ColumnCount = columnCount
	})
	if !found {
		return ErrNotFound
	}
	if tallySourceChanged {
		s.reconcileCameraTally(cam)
	}
	s.broadcast()
	s.onMutate()
	return nil
}

func (s *Store) RemoveCamera(id string) error {
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
func (s *Store) ActivePresetID(cam *Camera) *string {
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

// activePresetIDLocked reports which preset (if any) the camera is actually sitting on, per the
// cached value last written by recomputeActivePresetLocked.
func activePresetIDLocked(cam *Camera) *string {
	return cam.ActivePresetID
}

// recomputeActivePresetLocked refreshes cam.ActivePresetID from cam.CurrentPosition. Called
// whenever a fresh position reading comes in (RefreshPosition, finishTrigger) - never while a
// trigger is in flight, since triggerPreset clears ActivePresetID immediately and owns the
// position until the move settles.
func recomputeActivePresetLocked(cam *Camera) {
	cam.ActivePresetID = nil
	if cam.CurrentPosition == nil {
		return
	}
	for _, p := range cam.Presets {
		if closeEnough(*cam.CurrentPosition, p.Target) {
			id := p.ID
			cam.ActivePresetID = &id
			return
		}
	}
}

// ClientFor returns the PTZ client for a camera, safe to use concurrently with camera edits.
func (s *Store) ClientFor(cameraID string) (*ptz.Client, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return nil, false
	}
	return cam.Client, true
}

func (s *Store) PresetThumbnail(presetID string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.presetsByID[presetID]
	if !ok {
		return nil, false
	}
	return p.Thumbnail, true
}

var ErrNotFound = errors.New("not found")
