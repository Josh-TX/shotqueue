package state

import (
	"sync/atomic"
	"time"

	"shotqueue-backend/internal/ptz"
	"shotqueue-backend/internal/versions"
)

const (
	genSettleTimeout = 10 * time.Second
	genPollInterval  = 150 * time.Millisecond
)

// BuildSnapshot captures the current camera roster, presets and groups in the shape saved to a
// version. IDs are intentionally omitted (see internal/versions); group members reference presets
// by index instead.
func (s *Store) BuildSnapshot() []versions.VersionCamera {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]versions.VersionCamera, len(s.cameras))
	for i, cam := range s.cameras {
		presetIndex := make(map[string]int, len(cam.Presets))
		vc := versions.VersionCamera{
			Name:        cam.Name,
			Host:        cam.Host,
			Port:        cam.Port,
			TallySource: cam.TallySource,
			ColumnCount: cam.ColumnCount,
			Presets:     make([]versions.VersionPreset, len(cam.Presets)),
			Groups:      make([]versions.VersionGroup, len(cam.Groups)),
		}
		for j, p := range cam.Presets {
			presetIndex[p.ID] = j
			vc.Presets[j] = versions.VersionPreset{Name: p.Name, Target: p.Target}
		}
		for j, g := range cam.Groups {
			vg := versions.VersionGroup{Name: g.Name, Members: make([]int, 0, len(g.Members))}
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
// thumbnail generation is implicitly cancelled since it tracks cameras by the old IDs.
func (s *Store) LoadVersion(cams []versions.VersionCamera) error {
	s.genMu.Lock()
	s.genToken++
	s.genMu.Unlock()

	s.mu.Lock()
	newCameras := make([]*Camera, len(cams))
	presetsByID := make(map[string]*Preset)
	for i, vc := range cams {
		cam := &Camera{
			ID:          genID(),
			Name:        vc.Name,
			Host:        vc.Host,
			Port:        vc.Port,
			TallySource: vc.TallySource,
			ColumnCount: vc.ColumnCount,
			Client:      ptz.New(vc.Host, vc.Port),
			Status:      "none",
			nextGroupID: 1,
		}
		presetIDByIndex := make([]string, len(vc.Presets))
		for j, vp := range vc.Presets {
			p := &Preset{ID: genID(), Name: vp.Name, Target: vp.Target, ThumbnailVersion: 1}
			cam.Presets = append(cam.Presets, p)
			presetsByID[p.ID] = p
			presetIDByIndex[j] = p.ID
		}
		for _, vg := range vc.Groups {
			g := &Group{ID: cam.nextGroupID, Name: vg.Name}
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
	s.mu.Unlock()

	s.broadcast()
	s.onMutate()
	return nil
}

// genCounters aggregates the outcome of a StartGenThumbnails run across every camera's goroutine.
type genCounters struct {
	generated atomic.Int64
	skipped   atomic.Int64
	failed    atomic.Int64
}

// StartGenThumbnails re-triggers presets to (re)capture their thumbnails, one at a time per camera
// (so as not to fight over a single camera's position) but concurrently across cameras.
// allowLiveMove lets it move a camera that's currently live (normally blocked); includeExisting
// makes it redo presets that already have a thumbnail instead of only filling in missing ones.
// Starting a new run (or loading another version) cancels any run already in flight.
func (s *Store) StartGenThumbnails(allowLiveMove, includeExisting bool) {
	s.genMu.Lock()
	s.genToken++
	token := s.genToken
	s.genMu.Unlock()

	s.mu.Lock()
	cams := make([]*Camera, len(s.cameras))
	copy(cams, s.cameras)
	order := make(map[string][]string, len(cams))
	for _, cam := range cams {
		ids := genOrderLocked(cam)
		order[cam.ID] = ids
		cam.GenTotal = len(ids)
		cam.GenDone = 0
		cam.Generating = len(ids) > 0
	}
	s.mu.Unlock()
	s.broadcast()

	counters := &genCounters{}
	done := make(chan struct{})
	var running int
	for _, cam := range cams {
		ids := order[cam.ID]
		if len(ids) == 0 {
			continue
		}
		running++
		go func(cameraID string, ids []string) {
			s.genCamera(cameraID, ids, token, allowLiveMove, includeExisting, counters)
			done <- struct{}{}
		}(cam.ID, ids)
	}

	go func() {
		for i := 0; i < running; i++ {
			<-done
		}
		if s.genCancelled(token) {
			return
		}
		s.genComplete(int(counters.generated.Load()), int(counters.skipped.Load()), int(counters.failed.Load()))
	}()
}

func (s *Store) genCancelled(token int) bool {
	s.genMu.Lock()
	defer s.genMu.Unlock()
	return token != s.genToken
}

// genOrderLocked lists a camera's preset IDs to generate, with whichever preset is currently
// active (if any) moved to the front. Capturing the active preset doesn't require moving the
// camera at all, but only for as long as it stays active — so it has to happen before any other
// preset's move knocks the camera off of it. Caller must hold s.mu.
func genOrderLocked(cam *Camera) []string {
	ids := make([]string, len(cam.Presets))
	for i, p := range cam.Presets {
		ids[i] = p.ID
	}
	activeID := activePresetIDLocked(cam)
	if activeID == nil {
		return ids
	}
	for i, id := range ids {
		if id == *activeID {
			ids[0], ids[i] = ids[i], ids[0]
			break
		}
	}
	return ids
}

// finishPresetLocked advances a camera's gen progress by one preset, clearing Generating once
// every preset's been accounted for. Caller must hold s.mu.
func finishPresetLocked(cam *Camera) {
	cam.GenDone++
	if cam.GenDone >= cam.GenTotal {
		cam.Generating = false
	}
}

func (s *Store) genCamera(cameraID string, presetIDs []string, token int, allowLiveMove, includeExisting bool, counters *genCounters) {
	for _, presetID := range presetIDs {
		if s.genCancelled(token) {
			return
		}

		s.mu.Lock()
		cam := s.findCameraLocked(cameraID)
		if cam == nil {
			s.mu.Unlock()
			return
		}
		preset := s.findPresetLocked(cam, presetID)
		if preset == nil {
			// preset was deleted mid-run
			finishPresetLocked(cam)
			s.mu.Unlock()
			s.broadcast()
			continue
		}
		skip := (cam.Status == "live" && !allowLiveMove) || (!includeExisting && len(preset.Thumbnail) > 0)
		alreadyActive := !skip && func() bool {
			id := activePresetIDLocked(cam)
			return id != nil && *id == preset.ID
		}()
		client := cam.Client
		s.mu.Unlock()

		if skip {
			s.mu.Lock()
			if cam := s.findCameraLocked(cameraID); cam != nil {
				finishPresetLocked(cam)
			}
			s.mu.Unlock()
			counters.skipped.Add(1)
			s.broadcast()
			continue
		}

		// The camera never needs to (and, per isTriggerableLocked, isn't allowed to) "trigger" a
		// preset it's already sitting on — e.g. the preset that was active when the app booted.
		// Capture a snapshot directly instead of going through the normal move-and-settle flow.
		if alreadyActive {
			thumb, err := client.Snapshot()
			s.mu.Lock()
			cam = s.findCameraLocked(cameraID)
			if cam == nil {
				s.mu.Unlock()
				return
			}
			if p := s.presetsByID[presetID]; p != nil && err == nil {
				p.Thumbnail = thumb
				p.ThumbnailVersion++
				counters.generated.Add(1)
			} else {
				counters.failed.Add(1)
			}
			finishPresetLocked(cam)
			s.mu.Unlock()
			s.broadcast()
			continue
		}

		beforeVersion := preset.ThumbnailVersion
		s.triggerPreset(cameraID, presetID, allowLiveMove) // ignore error: e.g. already-triggering just means nothing to do

		settled := false
		deadline := time.Now().Add(genSettleTimeout)
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
			time.Sleep(genPollInterval)
		}

		s.mu.Lock()
		cam = s.findCameraLocked(cameraID)
		if cam == nil {
			s.mu.Unlock()
			return
		}
		if !settled {
			cam.Generating = false
			s.mu.Unlock()
			counters.failed.Add(1)
			s.broadcast()
			return
		}
		if p := s.presetsByID[presetID]; p != nil && p.ThumbnailVersion > beforeVersion {
			counters.generated.Add(1)
		} else {
			counters.failed.Add(1)
		}
		finishPresetLocked(cam)
		s.mu.Unlock()
		s.broadcast()
	}
}
