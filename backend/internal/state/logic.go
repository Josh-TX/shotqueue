package state

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"shotqueue-backend/internal/atem"
	"shotqueue-backend/internal/ptz"
)

const triggerPollInterval = 150 * time.Millisecond
const triggerTimeout = 5 * time.Second

// RefreshPosition live-queries the camera's actual position and updates the cached value used for
// activePresetId, so the idle poll loop (Store.Start) notices moves made by anything other than
// TriggerPreset (an external controller, a physical joystick, etc). A no-op while a trigger is in
// flight, since that goroutine owns the position until it settles.
func (s *Store) RefreshPosition(cameraID string) {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil || cam.TriggeringPresetID != nil {
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
	if cam := s.findCameraLocked(cameraID); cam != nil && cam.TriggeringPresetID == nil {
		cam.CurrentPosition = &pos
		recomputeActivePresetLocked(cam)
	}
	s.mu.Unlock()
}

// refreshPresetThumbnail captures a fresh snapshot for presetID, called by pollTick right after it
// notices the camera's position now matches that preset.
func (s *Store) refreshPresetThumbnail(cameraID, presetID string) {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
		return
	}
	client := cam.Client
	s.mu.Unlock()

	thumb, err := client.Snapshot()
	if err != nil {
		return
	}

	s.mu.Lock()
	if preset := s.presetsByID[presetID]; preset != nil {
		preset.Thumbnail = thumb
		preset.ThumbnailVersion++
	}
	s.mu.Unlock()
	s.broadcast()
}

// AddPreset captures the camera's current live position and a snapshot as a new preset.
func (s *Store) AddPreset(cameraID string, name string, groupIDs []int) (*Preset, error) {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
		return nil, newErr(404, "camera not found")
	}
	if cam.TriggeringPresetID != nil {
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
		ID:               genID(),
		Name:             name,
		Target:           pos,
		Thumbnail:        thumb,
		ThumbnailVersion: 1,
	}
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

// UpdatePresetPosition re-captures the camera's current live position and a snapshot into an
// existing preset, replacing its saved target and thumbnail.
func (s *Store) UpdatePresetPosition(cameraID, presetID string) error {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
		return newErr(404, "camera not found")
	}
	if cam.TriggeringPresetID != nil {
		s.mu.Unlock()
		return newErr(409, "camera is currently moving, no thumbnail to capture")
	}
	client := cam.Client
	s.mu.Unlock()

	pos, err := client.GetPosition()
	if err != nil {
		return newErr(502, "could not read camera position: %v", err)
	}
	thumb, err := client.Snapshot()
	if err != nil {
		return newErr(502, "could not capture snapshot: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	cam = s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	preset := s.presetsByID[presetID]
	if preset == nil {
		return newErr(404, "preset not found")
	}
	preset.Target = pos
	preset.Thumbnail = thumb
	preset.ThumbnailVersion++
	cam.CurrentPosition = &pos
	recomputeActivePresetLocked(cam)

	go s.broadcast()
	go s.onMutate()
	return nil
}

func (s *Store) RenamePreset(cameraID, presetID string, name string) error {
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

// ReorderPresets rearranges cam.Presets to match order, which must contain exactly the IDs of the
// camera's existing presets (in any order).
func (s *Store) ReorderPresets(cameraID string, order []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	if len(order) != len(cam.Presets) {
		return newErr(400, "order must include every preset exactly once")
	}
	byID := make(map[string]*Preset, len(cam.Presets))
	for _, p := range cam.Presets {
		byID[p.ID] = p
	}
	reordered := make([]*Preset, len(order))
	for i, id := range order {
		p, ok := byID[id]
		if !ok {
			return newErr(400, "order must include every preset exactly once")
		}
		reordered[i] = p
		delete(byID, id)
	}
	cam.Presets = reordered
	go s.broadcast()
	go s.onMutate()
	return nil
}

func (s *Store) DeletePreset(cameraID, presetID string) error {
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
	if cam.ActivePresetID != nil && *cam.ActivePresetID == presetID {
		cam.ActivePresetID = nil
	}
	go s.broadcast()
	go s.onMutate()
	return nil
}

func (s *Store) isTriggerableLocked(cam *Camera, preset *Preset, allowLive bool) error {
	if cam.Status == "live" && !allowLive {
		return newErr(409, "cannot trigger a preset while the camera is live")
	}
	if id := activePresetIDLocked(cam); id != nil && *id == preset.ID {
		return newErr(409, "preset is already active")
	}
	if cam.TriggeringPresetID != nil && *cam.TriggeringPresetID == preset.ID {
		return newErr(409, "preset is already triggering")
	}
	return nil
}

func (s *Store) findPresetLocked(cam *Camera, presetID string) *Preset {
	for _, p := range cam.Presets {
		if p.ID == presetID {
			return p
		}
	}
	return nil
}

func (s *Store) TriggerPreset(cameraID, presetID string) error {
	_, err := s.triggerPreset(cameraID, presetID, false)
	return err
}

// triggerPreset is TriggerPreset with an allowLive escape hatch, used only by genCamera when the
// "allow moving a live camera" option is enabled. It returns the TriggerGen it assigned, so a
// caller like genCamera can later tell whether a different trigger (e.g. a manual one) took over
// the camera before this one settled.
func (s *Store) triggerPreset(cameraID, presetID string, allowLive bool) (int, error) {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
		return 0, newErr(404, "camera not found")
	}
	preset := s.findPresetLocked(cam, presetID)
	if preset == nil {
		s.mu.Unlock()
		return 0, newErr(404, "preset not found")
	}
	if err := s.isTriggerableLocked(cam, preset, allowLive); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	if cam.Queued != nil && cam.Queued.PresetID == preset.ID {
		cam.Queued = nil
	}
	pid := preset.ID
	cam.TriggeringPresetID = &pid
	cam.ActivePresetID = nil
	cam.TriggerGen++
	gen := cam.TriggerGen
	client := cam.Client
	target := preset.Target
	s.mu.Unlock()

	s.broadcast()
	go s.finishTrigger(cameraID, presetID, client, target, gen)
	return gen, nil
}

// finishTrigger drives one preset move to completion. gen is the camera's TriggerGen at the moment
// this trigger was issued; if a newer trigger has since bumped it, this goroutine's results are
// stale (superseded by whatever the newer trigger is doing) and are discarded instead of being
// written back, so an in-flight trigger can never clobber a later one's preset/thumbnail/position.
func (s *Store) finishTrigger(cameraID, presetID string, client *ptz.Client, target ptz.Position, gen int) {
	if err := client.SetPositionRaw(target); err != nil {
		log.Printf("[state] camera %s: set position failed: %v", cameraID, err)
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
	if cam == nil || cam.TriggerGen != gen {
		// A newer trigger superseded this one; let it own the camera's state instead.
		s.mu.Unlock()
		return
	}
	cam.TriggeringPresetID = nil
	cam.CurrentPosition = &final
	recomputeActivePresetLocked(cam)
	if preset := s.presetsByID[presetID]; preset != nil && thumbErr == nil {
		preset.Thumbnail = thumb
		preset.ThumbnailVersion++
	}
	s.mu.Unlock()

	s.broadcast()
}

func (s *Store) QueuePreset(cameraID, presetID string, origin string) error {
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
	if cam.TriggeringPresetID != nil && *cam.TriggeringPresetID == preset.ID {
		return newErr(409, "cannot queue a triggering preset")
	}
	cam.Queued = &Queued{PresetID: preset.ID, Origin: origin}
	go s.broadcast()
	return nil
}

func (s *Store) UnqueuePreset(cameraID string) error {
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

func (s *Store) SetSelectedGroup(cameraID string, groupID *int) error {
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
// preset in the group carries a WasTaken bit, set (and reset once the whole group has cycled
// through) in markLive when it actually goes on air.
//
// This runs right after triggering the just-dequeued preset, before that preset has gone live (and
// so before its own WasTaken bit flips), so it's treated as taken here too via reservedID - both to
// exclude it from candidates and to detect a group that's completing its cycle right now.
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

	reservedID := cam.TriggeringPresetID
	if reservedID == nil {
		reservedID = activePresetIDLocked(cam)
	}

	if group.IsSequence {
		if next := s.sequenceNextLocked(cam, group, reservedID); next != nil {
			cam.Queued = &Queued{PresetID: *next, Origin: "auto"}
		}
		return
	}

	cycleComplete := true
	for _, presetID := range group.Members {
		preset := s.findPresetLocked(cam, presetID)
		taken := preset != nil && preset.WasTaken
		if reservedID != nil && *reservedID == presetID {
			taken = true
		}
		if !taken {
			cycleComplete = false
			break
		}
	}

	var candidates []string
	for _, presetID := range group.Members {
		if reservedID != nil && *reservedID == presetID {
			continue
		}
		preset := s.findPresetLocked(cam, presetID)
		if preset == nil {
			continue
		}
		if !cycleComplete && preset.WasTaken {
			continue
		}
		candidates = append(candidates, presetID)
	}
	if len(candidates) == 0 {
		return
	}
	cam.Queued = &Queued{PresetID: candidates[rand.Intn(len(candidates))], Origin: "auto"}
}

// sequenceNextLocked finds the group member that comes right after refID in the camera's preset
// (thumbnail) order, wrapping around to the start. refID need not itself be a group member.
// Returns nil if no member qualifies, e.g. a single-member group whose only member is refID itself.
func (s *Store) sequenceNextLocked(cam *Camera, group *Group, refID *string) *string {
	n := len(cam.Presets)
	if n == 0 {
		return nil
	}
	members := make(map[string]bool, len(group.Members))
	for _, id := range group.Members {
		members[id] = true
	}
	refIdx := -1
	if refID != nil {
		for i, p := range cam.Presets {
			if p.ID == *refID {
				refIdx = i
				break
			}
		}
	}
	for i := 1; i <= n; i++ {
		p := cam.Presets[(refIdx+i)%n]
		if !members[p.ID] {
			continue
		}
		if refID != nil && *refID == p.ID {
			return nil
		}
		id := p.ID
		return &id
	}
	return nil
}

func (s *Store) processOfflive(cameraID string) {
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

func (s *Store) UpdateGroup(cameraID string, groupID int, name *string, isSequence *bool) error {
	s.mu.Lock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		s.mu.Unlock()
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
		s.mu.Unlock()
		return newErr(404, "group not found")
	}
	if name != nil && *name != "" {
		group.Name = *name
	}
	if isSequence != nil && *isSequence != group.IsSequence {
		group.IsSequence = *isSequence
		if cam.SelectedGroupID != nil && *cam.SelectedGroupID == groupID {
			if cam.Queued != nil && cam.Queued.Origin == "auto" {
				cam.Queued = nil
			}
			s.autoQueueFillLocked(cam)
		}
	}
	s.mu.Unlock()
	go s.broadcast()
	go s.onMutate()
	return nil
}

// SetGroupCount resizes a camera's group list to exactly count groups (1..MaxGroupCount).
// Growing appends newly-named default groups ("Group N"); shrinking discards the trailing
// groups and their membership data.
func (s *Store) SetGroupCount(cameraID string, count int) error {
	if count < 1 || count > MaxGroupCount {
		return newErr(400, "group count must be between 1 and %d", MaxGroupCount)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return newErr(404, "camera not found")
	}
	if count > len(cam.Groups) {
		for i := len(cam.Groups); i < count; i++ {
			cam.Groups = append(cam.Groups, &Group{ID: cam.nextGroupID, Name: fmt.Sprintf("Group %d", i+1)})
			cam.nextGroupID++
		}
	} else if count < len(cam.Groups) {
		removedIDs := map[int]bool{}
		for _, g := range cam.Groups[count:] {
			removedIDs[g.ID] = true
		}
		cam.Groups = cam.Groups[:count]
		if cam.SelectedGroupID != nil && removedIDs[*cam.SelectedGroupID] {
			cam.SelectedGroupID = nil
			if cam.Queued != nil && cam.Queued.Origin == "auto" {
				cam.Queued = nil
			}
		}
	}
	go s.broadcast()
	go s.onMutate()
	return nil
}

func (s *Store) SetGroupMember(cameraID string, groupID int, presetID string, inGroup bool) error {
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
	s.lastTally = ts
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

// reconcileCameraTally recomputes a single camera's status against the last known tally
// snapshot. It's used when a camera's tally source is edited directly, since that doesn't
// come with a fresh ATEM tally event to trigger ApplyTally.
func (s *Store) reconcileCameraTally(cam *Camera) {
	s.mu.Lock()
	ts := s.lastTally
	from := cam.Status
	s.mu.Unlock()

	to := "none"
	for _, src := range ts.Live {
		if src == cam.TallySource {
			to = "live"
			break
		}
	}
	if to == "none" {
		for _, src := range ts.Preview {
			if src == cam.TallySource {
				to = "preview"
				break
			}
		}
	}
	if to == from {
		return
	}
	switch {
	case to == "live":
		s.markLive(cam)
	case from == "live":
		s.markOffLive(cam, to)
	default:
		s.mu.Lock()
		cam.Status = to
		s.mu.Unlock()
		s.broadcast()
	}
}

func (s *Store) markLive(cam *Camera) {
	s.mu.Lock()
	cam.Status = "live"
	activeID := activePresetIDLocked(cam)
	if activeID != nil {
		if preset := s.presetsByID[*activeID]; preset != nil {
			preset.WasTaken = true
			s.resetGroupCycleIfCompleteLocked(cam, preset.ID)
		}
	}
	s.mu.Unlock()
	s.broadcast()
}

// resetGroupCycleIfCompleteLocked clears WasTaken for every member of the camera's selected group
// once takenID (just marked taken) is the last one needed to complete the cycle, so the group
// starts a fresh round next time.
func (s *Store) resetGroupCycleIfCompleteLocked(cam *Camera, takenID string) {
	if cam.SelectedGroupID == nil {
		return
	}
	var group *Group
	for _, g := range cam.Groups {
		if g.ID == *cam.SelectedGroupID {
			group = g
			break
		}
	}
	if group == nil {
		return
	}
	inGroup := false
	for _, id := range group.Members {
		if id == takenID {
			inGroup = true
			break
		}
	}
	if !inGroup {
		return
	}
	for _, presetID := range group.Members {
		if preset := s.findPresetLocked(cam, presetID); preset == nil || !preset.WasTaken {
			return
		}
	}
	for _, presetID := range group.Members {
		if preset := s.findPresetLocked(cam, presetID); preset != nil {
			preset.WasTaken = false
		}
	}
}

func (s *Store) markOffLive(cam *Camera, to string) {
	s.mu.Lock()
	cam.Status = to
	cameraID := cam.ID
	s.mu.Unlock()
	s.broadcast()
	time.AfterFunc(50*time.Millisecond, func() { s.processOfflive(cameraID) })
}
