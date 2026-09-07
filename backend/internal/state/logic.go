package state

import (
	"log"
	"math/rand"
	"time"

	"shotqueue-backend/internal/atem"
	"shotqueue-backend/internal/ptz"
)

const triggerPollInterval = 150 * time.Millisecond
const triggerTimeout = 5 * time.Second

// RefreshPosition live-queries the camera's actual position and updates the cached value used for
// activePresetId, so polling /position notices moves made by anything other than TriggerPreset
// (an external controller, a physical joystick, etc). A no-op while a trigger is in flight, since
// that goroutine owns the position until it settles.
func (s *Store) RefreshPosition(cameraID int) {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil || cam.Triggering {
		s.mu.Unlock()
		return
	}
	client := cam.Client
	s.mu.Unlock()

	pos, err := client.GetPosition()
	if err != nil {
		return
	}

	s.mu.Lock()
	if cam := s.findCameraLocked(cameraID); cam != nil && !cam.Triggering {
		cam.CurrentPosition = &pos
	}
	s.mu.Unlock()
}

// AddPreset captures the camera's current live position and a snapshot as a new preset.
func (s *Store) AddPreset(cameraID int, name string, groupIDs []int) (*Preset, error) {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
		return nil, newErr(404, "camera not found")
	}
	if cam.Triggering {
		s.mu.Unlock()
		return nil, newErr(409, "camera is currently moving, no thumbnail to capture")
	}
	client := cam.Client
	s.mu.Unlock()

	pos, err := client.GetPosition()
	if err != nil {
		return nil, newErr(502, "could not read camera position: %v", err)
	}
	thumb, err := client.Snapshot()
	if err != nil {
		return nil, newErr(502, "could not capture snapshot: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	cam = s.findCameraLocked(cameraID)
	if cam == nil {
		return nil, newErr(404, "camera not found")
	}

	preset := &Preset{
		ID:               s.nextPreset,
		Name:             name,
		Target:           pos,
		Thumbnail:        thumb,
		ThumbnailVersion: 1,
	}
	s.nextPreset++
	cam.Presets = append(cam.Presets, preset)
	cam.CurrentPosition = &pos
	s.presetsByID[preset.ID] = preset

	for _, gid := range groupIDs {
		for _, g := range cam.Groups {
			if g.ID == gid {
				g.Members = append(g.Members, preset.ID)
			}
		}
	}

	go s.broadcast()
	go s.onMutate()
	return preset, nil
}

func (s *Store) RenamePreset(cameraID, presetID int, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	for _, p := range cam.Presets {
		if p.ID == presetID {
			p.Name = name
			go s.broadcast()
			go s.onMutate()
			return nil
		}
	}
	return newErr(404, "preset not found")
}

func (s *Store) DeletePreset(cameraID, presetID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	found := false
	out := cam.Presets[:0]
	for _, p := range cam.Presets {
		if p.ID == presetID {
			found = true
			delete(s.presetsByID, p.ID)
			continue
		}
		out = append(out, p)
	}
	if !found {
		return newErr(404, "preset not found")
	}
	cam.Presets = out
	for _, g := range cam.Groups {
		members := g.Members[:0]
		for _, id := range g.Members {
			if id != presetID {
				members = append(members, id)
			}
		}
		g.Members = members
	}
	if cam.Queued != nil && cam.Queued.PresetID == presetID {
		cam.Queued = nil
	}
	go s.broadcast()
	go s.onMutate()
	return nil
}

func (s *Store) isTriggerableLocked(cam *Camera, preset *Preset) error {
	if cam.Status == "live" {
		return newErr(409, "cannot trigger a preset while the camera is live")
	}
	if id := activePresetIDLocked(cam); id != nil && *id == preset.ID {
		return newErr(409, "preset is already active")
	}
	if cam.TriggeringPresetID == preset.ID {
		return newErr(409, "preset is already triggering")
	}
	return nil
}

func (s *Store) findPresetLocked(cam *Camera, presetID int) *Preset {
	for _, p := range cam.Presets {
		if p.ID == presetID {
			return p
		}
	}
	return nil
}

func (s *Store) TriggerPreset(cameraID, presetID int) error {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
		return newErr(404, "camera not found")
	}
	preset := s.findPresetLocked(cam, presetID)
	if preset == nil {
		s.mu.Unlock()
		return newErr(404, "preset not found")
	}
	if err := s.isTriggerableLocked(cam, preset); err != nil {
		s.mu.Unlock()
		return err
	}
	if cam.Queued != nil && cam.Queued.PresetID == preset.ID {
		cam.Queued = nil
	}
	cam.Triggering = true
	cam.TriggeringPresetID = preset.ID
	client := cam.Client
	target := preset.Target
	s.mu.Unlock()

	s.broadcast()
	go s.finishTrigger(cameraID, presetID, client, target)
	return nil
}

func (s *Store) finishTrigger(cameraID, presetID int, client *ptz.Client, target ptz.Position) {
	if err := client.SetPositionRaw(target); err != nil {
		log.Printf("[state] camera %d: set position failed: %v", cameraID, err)
	}

	deadline := time.Now().Add(triggerTimeout)
	var final ptz.Position = target
	for time.Now().Before(deadline) {
		pos, err := client.GetPosition()
		if err == nil {
			final = pos
			if closeEnough(pos, target) {
				break
			}
		}
		time.Sleep(triggerPollInterval)
	}

	thumb, thumbErr := client.Snapshot()

	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam != nil {
		cam.Triggering = false
		cam.TriggeringPresetID = 0
		cam.CurrentPosition = &final
	}
	if preset := s.presetsByID[presetID]; preset != nil && thumbErr == nil {
		preset.Thumbnail = thumb
		preset.ThumbnailVersion++
	}
	s.mu.Unlock()

	s.broadcast()
}

func (s *Store) QueuePreset(cameraID, presetID int, origin string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	preset := s.findPresetLocked(cam, presetID)
	if preset == nil {
		return newErr(404, "preset not found")
	}
	if id := activePresetIDLocked(cam); id != nil && *id == preset.ID {
		return newErr(409, "cannot queue the active preset")
	}
	if cam.TriggeringPresetID == preset.ID {
		return newErr(409, "cannot queue a triggering preset")
	}
	cam.Queued = &Queued{PresetID: preset.ID, Origin: origin}
	go s.broadcast()
	return nil
}

func (s *Store) UnqueuePreset(cameraID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	cam.Queued = nil
	go s.broadcast()
	return nil
}

func (s *Store) SetSelectedGroup(cameraID int, groupID *int) error {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
		return newErr(404, "camera not found")
	}
	prevQueued := cam.Queued
	cam.SelectedGroupID = groupID

	if groupID == nil {
		if prevQueued != nil && prevQueued.Origin == "auto" {
			cam.Queued = nil
		}
		s.mu.Unlock()
		s.broadcast()
		return nil
	}

	if prevQueued == nil || prevQueued.Origin == "auto" {
		cam.Queued = nil
		s.autoQueueFillLocked(cam)
	}
	s.mu.Unlock()
	s.broadcast()
	return nil
}

// autoQueueFillLocked picks the next preset to auto-queue from the camera's selected group. Each
// preset in the group carries a WasTriggered bit, set when it goes live; once every member of the
// group has been triggered, all their bits reset together so the cycle starts over.
func (s *Store) autoQueueFillLocked(cam *Camera) {
	if cam.SelectedGroupID == nil || cam.Queued != nil {
		return
	}
	var group *Group
	for _, g := range cam.Groups {
		if g.ID == *cam.SelectedGroupID {
			group = g
			break
		}
	}
	if group == nil || len(group.Members) == 0 {
		return
	}

	allTriggered := true
	for _, presetID := range group.Members {
		preset := s.findPresetLocked(cam, presetID)
		if preset == nil || !preset.WasTriggered {
			allTriggered = false
			break
		}
	}
	if allTriggered {
		for _, presetID := range group.Members {
			if preset := s.findPresetLocked(cam, presetID); preset != nil {
				preset.WasTriggered = false
			}
		}
	}

	activeID := activePresetIDLocked(cam)
	var candidates []int
	for _, presetID := range group.Members {
		preset := s.findPresetLocked(cam, presetID)
		if preset == nil || preset.WasTriggered {
			continue
		}
		if activeID != nil && *activeID == preset.ID {
			continue
		}
		if cam.TriggeringPresetID == preset.ID {
			continue
		}
		candidates = append(candidates, preset.ID)
	}
	if len(candidates) == 0 {
		return
	}
	cam.Queued = &Queued{PresetID: candidates[rand.Intn(len(candidates))], Origin: "auto"}
}

func (s *Store) processOfflive(cameraID int) {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
		return
	}
	queued := cam.Queued
	s.mu.Unlock()

	if queued != nil {
		if err := s.TriggerPreset(cameraID, queued.PresetID); err != nil {
			s.mu.Lock()
			if cam.Queued != nil && cam.Queued.PresetID == queued.PresetID {
				cam.Queued = nil
			}
			s.mu.Unlock()
		}
	}

	s.mu.Lock()
	s.autoQueueFillLocked(cam)
	s.mu.Unlock()
	s.broadcast()
}

// ---- groups ----

func (s *Store) AddGroup(cameraID int, name string) (*Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return nil, newErr(404, "camera not found")
	}
	used := map[string]bool{}
	for _, g := range cam.Groups {
		used[g.Color] = true
	}
	color := GroupColors[0]
	for _, c := range GroupColors {
		if !used[c] {
			color = c
			break
		}
	}
	group := &Group{ID: cam.nextGroupID, Name: name, Color: color}
	cam.nextGroupID++
	cam.Groups = append(cam.Groups, group)
	go s.broadcast()
	go s.onMutate()
	return group, nil
}

func (s *Store) UpdateGroup(cameraID, groupID int, name, color string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	for _, g := range cam.Groups {
		if g.ID == groupID {
			if name != "" {
				g.Name = name
			}
			if color != "" {
				g.Color = color
			}
			go s.broadcast()
			go s.onMutate()
			return nil
		}
	}
	return newErr(404, "group not found")
}

func (s *Store) DeleteGroup(cameraID, groupID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	found := false
	out := cam.Groups[:0]
	for _, g := range cam.Groups {
		if g.ID == groupID {
			found = true
			continue
		}
		out = append(out, g)
	}
	if !found {
		return newErr(404, "group not found")
	}
	cam.Groups = out
	if cam.SelectedGroupID != nil && *cam.SelectedGroupID == groupID {
		cam.SelectedGroupID = nil
		if cam.Queued != nil && cam.Queued.Origin == "auto" {
			cam.Queued = nil
		}
	}
	go s.broadcast()
	go s.onMutate()
	return nil
}

func (s *Store) SetGroupMember(cameraID, groupID, presetID int, inGroup bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	var group *Group
	for _, g := range cam.Groups {
		if g.ID == groupID {
			group = g
			break
		}
	}
	if group == nil {
		return newErr(404, "group not found")
	}
	if s.findPresetLocked(cam, presetID) == nil {
		return newErr(404, "preset not found")
	}

	existingIdx := -1
	for i, id := range group.Members {
		if id == presetID {
			existingIdx = i
			break
		}
	}
	if !inGroup {
		if existingIdx >= 0 {
			group.Members = append(group.Members[:existingIdx], group.Members[existingIdx+1:]...)
		}
	} else if existingIdx < 0 {
		group.Members = append(group.Members, presetID)
	}
	go s.broadcast()
	go s.onMutate()
	return nil
}

// ---- ATEM tally wiring ----

// ApplyTally reconciles a new tally snapshot against the camera roster. Cameras leaving "live",
// after a short grace period (mirroring the old fake-ATEM's crossfade pause), fire any queued
// preset and refill the group's auto-queue.
func (s *Store) ApplyTally(ts atem.TallyState) {
	liveSet := map[uint16]bool{}
	for _, src := range ts.Live {
		liveSet[src] = true
	}
	previewSet := map[uint16]bool{}
	for _, src := range ts.Preview {
		previewSet[src] = true
	}

	type change struct {
		cam  *Camera
		from string
		to   string
	}
	var changes []change

	s.mu.Lock()
	for _, cam := range s.cameras {
		to := "none"
		if liveSet[cam.TallySource] {
			to = "live"
		} else if previewSet[cam.TallySource] {
			to = "preview"
		}
		if to != cam.Status {
			changes = append(changes, change{cam: cam, from: cam.Status, to: to})
		}
	}
	s.mu.Unlock()

	for _, ch := range changes {
		switch {
		case ch.to == "live":
			s.markLive(ch.cam)
		case ch.from == "live":
			s.markOffLive(ch.cam, ch.to)
		default:
			s.mu.Lock()
			ch.cam.Status = ch.to
			s.mu.Unlock()
			s.broadcast()
		}
	}
}

func (s *Store) markLive(cam *Camera) {
	s.mu.Lock()
	cam.Status = "live"
	activeID := activePresetIDLocked(cam)
	if activeID != nil {
		if preset := s.presetsByID[*activeID]; preset != nil {
			preset.WasTriggered = true
		}
	}
	s.mu.Unlock()
	s.broadcast()
}

func (s *Store) markOffLive(cam *Camera, to string) {
	s.mu.Lock()
	cam.Status = to
	cameraID := cam.ID
	s.mu.Unlock()
	s.broadcast()
	time.AfterFunc(50*time.Millisecond, func() { s.processOfflive(cameraID) })
}
