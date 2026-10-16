package state

import (
	"fmt"
	"log"

	"shotqueue-backend/internal/config"
)

// Scene is a macro: executing it applies every camera's entry at once (visibility, auto-queue
// group, and optionally a preset to trigger and/or queue). It is not a state - nothing tracks
// which scene is "current".
type Scene struct {
	ID      string
	Name    string
	Cameras []*SceneCamera // one entry per camera, sorted by CameraNum (see ensureSceneEntriesLocked)
}

type SceneCamera struct {
	CameraNum       int
	IsHidden        bool
	GroupID         *int    // auto-queue group; nil = auto-queue off
	TriggerPresetID *string // nil = don't trigger anything
	QueuePresetID   *string // nil = leave the camera's queue alone
}

type SceneCameraDTO struct {
	CameraNum       int     `json:"cameraNum"`
	IsHidden        bool    `json:"isHidden"`
	GroupID         *int    `json:"groupId"`
	TriggerPresetID *string `json:"triggerPresetId"`
	QueuePresetID   *string `json:"queuePresetId"`
}

type SceneDTO struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Cameras []SceneCameraDTO `json:"cameras"`
}

func copyStr(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func copyInt(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// PublicScenes returns a JSON-safe snapshot of every scene, in display order.
func (s *Store) PublicScenes() []SceneDTO {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SceneDTO, len(s.scenes))
	for i, sc := range s.scenes {
		cams := make([]SceneCameraDTO, len(sc.Cameras))
		for j, c := range sc.Cameras {
			cams[j] = SceneCameraDTO{
				CameraNum:       c.CameraNum,
				IsHidden:        c.IsHidden,
				GroupID:         copyInt(c.GroupID),
				TriggerPresetID: copyStr(c.TriggerPresetID),
				QueuePresetID:   copyStr(c.QueuePresetID),
			}
		}
		out[i] = SceneDTO{ID: sc.ID, Name: sc.Name, Cameras: cams}
	}
	return out
}

func (s *Store) findSceneLocked(id string) *Scene {
	for _, sc := range s.scenes {
		if sc.ID == id {
			return sc
		}
	}
	return nil
}

// ensureSceneEntriesLocked makes every scene have exactly one entry per current camera (sorted by
// cameraNum): entries for removed cameras are dropped, and new cameras get defaults - the camera's
// current visibility and auto-queue off. Call after anything that changes the camera set.
func (s *Store) ensureSceneEntriesLocked() {
	for _, sc := range s.scenes {
		existing := make(map[int]*SceneCamera, len(sc.Cameras))
		for _, c := range sc.Cameras {
			existing[c.CameraNum] = c
		}
		out := make([]*SceneCamera, 0, len(s.cameras))
		for _, cam := range s.cameras {
			if c, ok := existing[cam.CameraNum]; ok {
				out = append(out, c)
			} else {
				out = append(out, &SceneCamera{CameraNum: cam.CameraNum, IsHidden: cam.IsHidden})
			}
		}
		sc.Cameras = out
	}
}

// scrubScenePresetLocked removes a deleted preset from every scene's trigger/queue on that camera.
func (s *Store) scrubScenePresetLocked(cameraNum int, presetID string) {
	for _, sc := range s.scenes {
		for _, c := range sc.Cameras {
			if c.CameraNum != cameraNum {
				continue
			}
			if c.TriggerPresetID != nil && *c.TriggerPresetID == presetID {
				c.TriggerPresetID = nil
			}
			if c.QueuePresetID != nil && *c.QueuePresetID == presetID {
				c.QueuePresetID = nil
			}
		}
	}
}

// scrubSceneGroupLocked turns auto-queue off in every scene that selected a now-removed group.
func (s *Store) scrubSceneGroupLocked(cameraNum int, groupID int) {
	for _, sc := range s.scenes {
		for _, c := range sc.Cameras {
			if c.CameraNum == cameraNum && c.GroupID != nil && *c.GroupID == groupID {
				c.GroupID = nil
			}
		}
	}
}

func (s *Store) AddScene() SceneDTO {
	var id string
	s.withLock(func() {
		sc := &Scene{ID: genID(), Name: fmt.Sprintf("Scene %d", len(s.scenes)+1)}
		s.scenes = append(s.scenes, sc)
		s.ensureSceneEntriesLocked()
		id = sc.ID
	})
	s.broadcast()
	s.onMutate()
	return s.sceneDTO(id)
}

func (s *Store) sceneDTO(id string) SceneDTO {
	for _, d := range s.PublicScenes() {
		if d.ID == id {
			return d
		}
	}
	return SceneDTO{}
}

func (s *Store) DuplicateScene(id string) (SceneDTO, error) {
	var newID string
	var err error
	s.withLock(func() {
		src := s.findSceneLocked(id)
		if src == nil {
			err = newErr(404, "scene not found")
			return
		}
		dup := &Scene{ID: genID(), Name: src.Name + " copy"}
		for _, c := range src.Cameras {
			cc := *c
			cc.GroupID = copyInt(c.GroupID)
			cc.TriggerPresetID = copyStr(c.TriggerPresetID)
			cc.QueuePresetID = copyStr(c.QueuePresetID)
			dup.Cameras = append(dup.Cameras, &cc)
		}
		// insert right after the original
		idx := 0
		for i, sc := range s.scenes {
			if sc.ID == id {
				idx = i + 1
			}
		}
		s.scenes = append(s.scenes[:idx], append([]*Scene{dup}, s.scenes[idx:]...)...)
		newID = dup.ID
	})
	if err != nil {
		return SceneDTO{}, err
	}
	s.broadcast()
	s.onMutate()
	return s.sceneDTO(newID), nil
}

func (s *Store) RenameScene(id, name string) error {
	if name == "" {
		return newErr(400, "name is required")
	}
	var found bool
	s.withLock(func() {
		if sc := s.findSceneLocked(id); sc != nil {
			sc.Name = name
			found = true
		}
	})
	if !found {
		return newErr(404, "scene not found")
	}
	s.broadcast()
	s.onMutate()
	return nil
}

func (s *Store) DeleteScene(id string) error {
	var found bool
	s.withLock(func() {
		for i, sc := range s.scenes {
			if sc.ID == id {
				s.scenes = append(s.scenes[:i], s.scenes[i+1:]...)
				found = true
				return
			}
		}
	})
	if !found {
		return newErr(404, "scene not found")
	}
	s.broadcast()
	s.onMutate()
	return nil
}

// ReorderScenes rearranges the scene list to match order, which must contain every scene ID once.
func (s *Store) ReorderScenes(order []string) error {
	var err error
	s.withLock(func() {
		if len(order) != len(s.scenes) {
			err = newErr(400, "order must include every scene exactly once")
			return
		}
		byID := make(map[string]*Scene, len(s.scenes))
		for _, sc := range s.scenes {
			byID[sc.ID] = sc
		}
		reordered := make([]*Scene, len(order))
		for i, id := range order {
			sc, ok := byID[id]
			if !ok {
				err = newErr(400, "order must include every scene exactly once")
				return
			}
			reordered[i] = sc
			delete(byID, id)
		}
		s.scenes = reordered
	})
	if err != nil {
		return err
	}
	s.broadcast()
	s.onMutate()
	return nil
}

// SceneCameraPatch is a partial update of one camera's entry in a scene. For the nullable fields,
// the Set flag says whether to apply the value (nil value = clear).
type SceneCameraPatch struct {
	IsHidden           *bool
	SetGroup           bool
	GroupID            *int
	SetTriggerPresetID bool
	TriggerPresetID    *string
	SetQueuePresetID   bool
	QueuePresetID      *string
}

func (s *Store) UpdateSceneCamera(sceneID string, cameraNum int, p SceneCameraPatch) error {
	var err error
	s.withLock(func() {
		sc := s.findSceneLocked(sceneID)
		if sc == nil {
			err = newErr(404, "scene not found")
			return
		}
		cam := s.findCameraLocked(cameraNum)
		if cam == nil {
			err = newErr(404, "camera not found")
			return
		}
		var entry *SceneCamera
		for _, c := range sc.Cameras {
			if c.CameraNum == cameraNum {
				entry = c
			}
		}
		if entry == nil {
			err = newErr(404, "camera not in scene")
			return
		}
		if p.SetGroup && p.GroupID != nil {
			found := false
			for _, g := range cam.Groups {
				if g.ID == *p.GroupID {
					found = true
				}
			}
			if !found {
				err = newErr(404, "group not found")
				return
			}
		}
		if p.SetTriggerPresetID && p.TriggerPresetID != nil && s.findPresetLocked(cam, *p.TriggerPresetID) == nil {
			err = newErr(404, "preset not found")
			return
		}
		if p.SetQueuePresetID && p.QueuePresetID != nil && s.findPresetLocked(cam, *p.QueuePresetID) == nil {
			err = newErr(404, "preset not found")
			return
		}
		if p.IsHidden != nil {
			entry.IsHidden = *p.IsHidden
		}
		if p.SetGroup {
			entry.GroupID = p.GroupID
		}
		if p.SetTriggerPresetID {
			entry.TriggerPresetID = p.TriggerPresetID
		}
		if p.SetQueuePresetID {
			entry.QueuePresetID = p.QueuePresetID
		}
	})
	if err != nil {
		return err
	}
	s.broadcast()
	s.onMutate()
	return nil
}

// ExecuteScene applies every camera's entry. Per camera, in order: visibility; auto-queue group
// (changing it drops/refills an auto-queued preset as usual; unchanged is left alone); trigger
// (a live camera can't be triggered, so the preset is queued instead and the scene's own queue
// is ignored); queue (replaces whatever was queued). A camera the scene hides gets only the
// visibility change. Per-camera failures (e.g. preset already active) are logged and skipped.
func (s *Store) ExecuteScene(id string) error {
	s.mu.Lock()
	sc := s.findSceneLocked(id)
	if sc == nil {
		s.mu.Unlock()
		return newErr(404, "scene not found")
	}
	entries := make([]SceneCamera, len(sc.Cameras))
	for i, c := range sc.Cameras {
		entries[i] = SceneCamera{
			CameraNum:       c.CameraNum,
			IsHidden:        c.IsHidden,
			GroupID:         copyInt(c.GroupID),
			TriggerPresetID: copyStr(c.TriggerPresetID),
			QueuePresetID:   copyStr(c.QueuePresetID),
		}
	}
	s.mu.Unlock()

	for _, e := range entries {
		s.mu.Lock()
		cam := s.findCameraLocked(e.CameraNum)
		if cam == nil {
			s.mu.Unlock()
			continue
		}
		hiddenChanged := cam.IsHidden != e.IsHidden
		groupChanged := (cam.SelectedGroupID == nil) != (e.GroupID == nil) ||
			(cam.SelectedGroupID != nil && e.GroupID != nil && *cam.SelectedGroupID != *e.GroupID)
		s.mu.Unlock()

		if hiddenChanged {
			if err := s.SetHidden(e.CameraNum, e.IsHidden); err != nil {
				log.Printf("[state] scene %s camera %d: set hidden failed: %v", id, e.CameraNum, err)
			}
		}
		if e.IsHidden {
			continue
		}
		if groupChanged {
			if err := s.SetSelectedGroup(e.CameraNum, e.GroupID); err != nil {
				log.Printf("[state] scene %s camera %d: set auto-queue failed: %v", id, e.CameraNum, err)
			}
		}

		queueID := e.QueuePresetID
		if e.TriggerPresetID != nil {
			s.mu.Lock()
			cam = s.findCameraLocked(e.CameraNum)
			live := cam != nil && cam.Status == "live"
			s.mu.Unlock()
			if live {
				queueID = e.TriggerPresetID
			} else if err := s.TriggerPreset(e.CameraNum, *e.TriggerPresetID); err != nil {
				log.Printf("[state] scene %s camera %d: trigger failed: %v", id, e.CameraNum, err)
			}
		}
		if queueID != nil {
			if err := s.QueuePreset(e.CameraNum, *queueID, "manual"); err != nil {
				log.Printf("[state] scene %s camera %d: queue failed: %v", id, e.CameraNum, err)
			}
		}
	}
	return nil
}

// sceneSnapshotLocked converts scenes to their saved form, with presets/groups as indices into the
// per-camera maps built by BuildSnapshot.
func (s *Store) sceneSnapshotLocked(presetIndex map[int]map[string]int, groupIndex map[int]map[int]int) []config.ConfigScene {
	if len(s.scenes) == 0 {
		return nil
	}
	out := make([]config.ConfigScene, len(s.scenes))
	for i, sc := range s.scenes {
		cs := config.ConfigScene{Name: sc.Name, Cameras: make([]config.ConfigSceneCamera, 0, len(sc.Cameras))}
		for _, c := range sc.Cameras {
			cc := config.ConfigSceneCamera{CameraNum: c.CameraNum, IsHidden: c.IsHidden}
			if c.GroupID != nil {
				if idx, ok := groupIndex[c.CameraNum][*c.GroupID]; ok {
					cc.GroupIndex = &idx
				}
			}
			if c.TriggerPresetID != nil {
				if idx, ok := presetIndex[c.CameraNum][*c.TriggerPresetID]; ok {
					cc.Trigger = &idx
				}
			}
			if c.QueuePresetID != nil {
				if idx, ok := presetIndex[c.CameraNum][*c.QueuePresetID]; ok {
					cc.Queue = &idx
				}
			}
			cs.Cameras = append(cs.Cameras, cc)
		}
		out[i] = cs
	}
	return out
}

// loadScenesLocked replaces the scene list from saved form. Must run after s.cameras is set (group
// IDs are assigned 1..n in order of the camera's saved groups, so a group index i is ID i+1).
func (s *Store) loadScenesLocked(scenes []config.ConfigScene, presetIDsByCam map[int][]string) {
	s.scenes = nil
	for _, cs := range scenes {
		sc := &Scene{ID: genID(), Name: cs.Name}
		for _, cc := range cs.Cameras {
			cam := s.findCameraLocked(cc.CameraNum)
			if cam == nil {
				continue
			}
			c := &SceneCamera{CameraNum: cc.CameraNum, IsHidden: cc.IsHidden}
			if cc.GroupIndex != nil && *cc.GroupIndex >= 0 && *cc.GroupIndex < len(cam.Groups) {
				id := cam.Groups[*cc.GroupIndex].ID
				c.GroupID = &id
			}
			ids := presetIDsByCam[cc.CameraNum]
			if cc.Trigger != nil && *cc.Trigger >= 0 && *cc.Trigger < len(ids) {
				id := ids[*cc.Trigger]
				c.TriggerPresetID = &id
			}
			if cc.Queue != nil && *cc.Queue >= 0 && *cc.Queue < len(ids) {
				id := ids[*cc.Queue]
				c.QueuePresetID = &id
			}
			sc.Cameras = append(sc.Cameras, c)
		}
		s.scenes = append(s.scenes, sc)
	}
	s.ensureSceneEntriesLocked()
}
