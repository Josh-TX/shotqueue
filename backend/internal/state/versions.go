package state

import (
	"time"

	"shotqueue-backend/internal/ptz"
	"shotqueue-backend/internal/versions"
)

const (
	regenSettleTimeout = 10 * time.Second
	regenPollInterval  = 150 * time.Millisecond
)

// BuildSnapshot captures the current camera roster, presets and groups in the shape saved to a
// version. IDs are intentionally omitted (see internal/versions); group members reference presets
// by index instead.
func (s *Store) BuildSnapshot() []versions.VersionCamera {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]versions.VersionCamera, len(s.cameras))
	for i, cam := range s.cameras {
		presetIndex := make(map[int]int, len(cam.Presets))
		vc := versions.VersionCamera{
			Name:        cam.Name,
			Host:        cam.Host,
			Port:        cam.Port,
			TallySource: cam.TallySource,
			Presets:     make([]versions.VersionPreset, len(cam.Presets)),
			Groups:      make([]versions.VersionGroup, len(cam.Groups)),
		}
		for j, p := range cam.Presets {
			presetIndex[p.ID] = j
			vc.Presets[j] = versions.VersionPreset{Name: p.Name, Target: p.Target}
		}
		for j, g := range cam.Groups {
			vg := versions.VersionGroup{Name: g.Name, Color: g.Color, Members: make([]int, 0, len(g.Members))}
			for _, presetID := range g.Members {
				if idx, ok := presetIndex[presetID]; ok {
					vg.Members = append(vg.Members, idx)
				}
			}
			vc.Groups[j] = vg
		}
		out[i] = vc
	}
	return out
}

// LoadVersion fully replaces the camera roster and every camera's presets and groups (all in
// memory only) with the given snapshot. Cameras/presets/groups all get fresh IDs; any in-flight
// thumbnail regeneration is implicitly cancelled since it tracks cameras by the old IDs.
func (s *Store) LoadVersion(cams []versions.VersionCamera) error {
	s.regenMu.Lock()
	s.regenToken++
	s.regenMu.Unlock()

	s.mu.Lock()
	newCameras := make([]*Camera, len(cams))
	presetsByID := make(map[int]*Preset)
	nextPreset := s.nextPreset
	nextCameraID := s.nextCameraID
	for i, vc := range cams {
		cam := &Camera{
			ID:          nextCameraID,
			Name:        vc.Name,
			Host:        vc.Host,
			Port:        vc.Port,
			TallySource: vc.TallySource,
			Client:      ptz.New(vc.Host, vc.Port),
			Status:      "none",
			nextGroupID: 1,
		}
		nextCameraID++
		presetIDByIndex := make([]int, len(vc.Presets))
		for j, vp := range vc.Presets {
			p := &Preset{ID: nextPreset, Name: vp.Name, Target: vp.Target, ThumbnailVersion: 1}
			nextPreset++
			cam.Presets = append(cam.Presets, p)
			presetsByID[p.ID] = p
			presetIDByIndex[j] = p.ID
		}
		for _, vg := range vc.Groups {
			g := &Group{ID: cam.nextGroupID, Name: vg.Name, Color: vg.Color}
			cam.nextGroupID++
			for _, idx := range vg.Members {
				if idx >= 0 && idx < len(presetIDByIndex) {
					g.Members = append(g.Members, presetIDByIndex[idx])
				}
			}
			cam.Groups = append(cam.Groups, g)
		}
		newCameras[i] = cam
	}
	s.cameras = newCameras
	s.presetsByID = presetsByID
	s.nextPreset = nextPreset
	s.nextCameraID = nextCameraID
	s.mu.Unlock()

	s.broadcast()
	s.onMutate()
	return nil
}

// StartRegenThumbnails re-triggers every preset on every camera, one at a time per camera (so as
// not to fight over a single camera's position) but concurrently across cameras, so that each
// preset's normal trigger-and-settle flow recaptures its thumbnail. Starting a new regen (or
// loading another version) cancels any regen already in flight.
func (s *Store) StartRegenThumbnails() {
	s.regenMu.Lock()
	s.regenToken++
	token := s.regenToken
	s.regenMu.Unlock()

	s.mu.Lock()
	cams := make([]*Camera, len(s.cameras))
	copy(cams, s.cameras)
	for _, cam := range cams {
		cam.RegenTotal = len(cam.Presets)
		cam.RegenDone = 0
		cam.Regenerating = len(cam.Presets) > 0
	}
	s.mu.Unlock()
	s.broadcast()

	for _, cam := range cams {
		if len(cam.Presets) == 0 {
			continue
		}
		go s.regenCamera(cam.ID, token)
	}
}

func (s *Store) regenCancelled(token int) bool {
	s.regenMu.Lock()
	defer s.regenMu.Unlock()
	return token != s.regenToken
}

func (s *Store) regenCamera(cameraID, token int) {
	for {
		if s.regenCancelled(token) {
			return
		}

		s.mu.Lock()
		cam := s.findCameraLocked(cameraID)
		if cam == nil {
			s.mu.Unlock()
			return
		}
		if cam.RegenDone >= len(cam.Presets) {
			cam.Regenerating = false
			s.mu.Unlock()
			s.broadcast()
			return
		}
		presetID := cam.Presets[cam.RegenDone].ID
		s.mu.Unlock()

		s.TriggerPreset(cameraID, presetID) // ignore error: e.g. already-active just means nothing to do

		settled := false
		deadline := time.Now().Add(regenSettleTimeout)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			c := s.findCameraLocked(cameraID)
			stillTriggering := c != nil && c.Triggering
			s.mu.Unlock()
			if c == nil {
				return
			}
			if !stillTriggering {
				settled = true
				break
			}
			time.Sleep(regenPollInterval)
		}

		s.mu.Lock()
		cam = s.findCameraLocked(cameraID)
		if cam == nil {
			s.mu.Unlock()
			return
		}
		if !settled {
			cam.Regenerating = false
			s.mu.Unlock()
			s.broadcast()
			return
		}
		cam.RegenDone++
		if cam.RegenDone >= cam.RegenTotal {
			cam.Regenerating = false
		}
		s.mu.Unlock()
		s.broadcast()
	}
}
