package state

import (
	"fmt"
	"sort"
	"time"

	"shotqueue-backend/internal/config"
	"shotqueue-backend/internal/ptz"
	"shotqueue-backend/internal/settings"
)

const (
	genSettleTimeout = 10 * time.Second
	genPollInterval  = 150 * time.Millisecond
)

// BuildSnapshot captures the current camera roster, presets and groups in the shape saved to a
// config. IDs are intentionally omitted (see internal/config); group members reference presets
// by index instead.
func (s *Store) BuildSnapshot() []config.ConfigCamera {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]config.ConfigCamera, len(s.cameras))
	for i, cam := range s.cameras {
		presetIndex := make(map[string]int, len(cam.Presets))
		vc := config.ConfigCamera{
			CameraNum:   cam.CameraNum,
			ColumnCount: cam.ColumnCount,
			IsHidden:    cam.IsHidden,
			Presets:     make([]config.ConfigPreset, len(cam.Presets)),
			Groups:      make([]config.ConfigGroup, len(cam.Groups)),
		}
		for j, p := range cam.Presets {
			presetIndex[p.ID] = j
			vc.Presets[j] = config.ConfigPreset{Name: p.Name, Target: p.Target}
		}
		for j, g := range cam.Groups {
			vg := config.ConfigGroup{Name: g.Name, Members: make([]int, 0, len(g.Members)), IsSequence: g.IsSequence}
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

// LoadSeed is one camera's full state as loaded from a config, joined with its connection info
// from settings (see BuildLoadSeeds).
type LoadSeed struct {
	CameraNum   int
	Host        string
	Port        string
	Username    string
	Password    string
	ColumnCount int
	IsHidden    bool
	Presets     []config.ConfigPreset
	Groups      []config.ConfigGroup
}

// BuildLoadSeeds joins a config's camera list against the current settings roster, sorted by
// cameraNum: cameraNums no longer present in settings are dropped (returned separately, for
// logging), and cameras in settings but absent from the config are added hidden and empty (default
// columnCount and groups) so they still show up in the visibility sidebar.
func BuildLoadSeeds(cams []config.ConfigCamera, settingsStore *settings.Store) (seeds []LoadSeed, dropped []int) {
	seen := make(map[int]bool, len(cams))
	for _, cc := range cams {
		cs, ok := settingsStore.CameraByNum(cc.CameraNum)
		if !ok {
			dropped = append(dropped, cc.CameraNum)
			continue
		}
		seen[cc.CameraNum] = true
		seeds = append(seeds, LoadSeed{
			CameraNum:   cs.CameraNum,
			Host:        cs.Host,
			Port:        cs.Port,
			Username:    cs.Username,
			Password:    cs.Password,
			ColumnCount: cc.ColumnCount,
			IsHidden:    cc.IsHidden,
			Presets:     cc.Presets,
			Groups:      cc.Groups,
		})
	}
	for _, cs := range settingsStore.Cameras() {
		if seen[cs.CameraNum] {
			continue
		}
		groups := make([]config.ConfigGroup, DefaultGroupCount)
		for i := range groups {
			groups[i] = config.ConfigGroup{Name: fmt.Sprintf("Group %d", i+1), Members: []int{}}
		}
		seeds = append(seeds, LoadSeed{
			CameraNum:   cs.CameraNum,
			Host:        cs.Host,
			Port:        cs.Port,
			Username:    cs.Username,
			Password:    cs.Password,
			ColumnCount: DefaultColumnCount,
			IsHidden:    true,
			Groups:      groups,
		})
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].CameraNum < seeds[j].CameraNum })
	return seeds, dropped
}

// nearestThumbnail returns the thumbnail of the preset whose target is closest to target (by
// largest per-axis delta) among those within positionTolerance, or nil if none has one.
func nearestThumbnail(presets []*Preset, target ptz.Position) []byte {
	var best []byte
	bestDist := positionTolerance + 1
	for _, p := range presets {
		if len(p.Thumbnail) == 0 || !closeEnough(p.Target, target) {
			continue
		}
		d := max(abs(p.Target.Pan-target.Pan), abs(p.Target.Tilt-target.Tilt), abs(p.Target.Zoom-target.Zoom))
		if d < bestDist {
			best, bestDist = p.Thumbnail, d
		}
	}
	return best
}

// LoadConfig fully replaces the camera roster and every camera's presets and groups (all in
// memory only) with the given seeds. Presets/groups all get fresh IDs; any in-flight thumbnail
// generation is implicitly cancelled since it tracks presets by the old IDs. A new preset inherits
// the thumbnail of the nearest current preset on the same camera within positionTolerance.
func (s *Store) LoadConfig(seeds []LoadSeed) error {
	s.genMu.Lock()
	s.genToken++
	s.genMu.Unlock()

	s.mu.Lock()
	oldPresets := make(map[int][]*Preset, len(s.cameras))
	for _, cam := range s.cameras {
		oldPresets[cam.CameraNum] = cam.Presets
	}
	newCameras := make([]*Camera, len(seeds))
	presetsByID := make(map[string]*Preset)
	for i, seed := range seeds {
		cam := &Camera{
			CameraNum:   seed.CameraNum,
			ColumnCount: seed.ColumnCount,
			IsHidden:    seed.IsHidden,
			Client:      ptz.New(seed.Host, seed.Port, seed.Username, seed.Password),
			Status:      "none",
			nextGroupID: 1,
		}
		presetIDByIndex := make([]string, len(seed.Presets))
		for j, vp := range seed.Presets {
			p := &Preset{ID: genID(), Name: vp.Name, Target: vp.Target, ThumbnailVersion: 1}
			p.Thumbnail = nearestThumbnail(oldPresets[seed.CameraNum], vp.Target)
			cam.Presets = append(cam.Presets, p)
			presetsByID[p.ID] = p
			presetIDByIndex[j] = p.ID
		}
		for _, vg := range seed.Groups {
			g := &Group{ID: cam.nextGroupID, Name: vg.Name, IsSequence: vg.IsSequence}
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
	sortCameras(newCameras)
	s.cameras = newCameras
	s.presetsByID = presetsByID
	s.mu.Unlock()

	s.broadcast()
	s.onMutate()
	return nil
}

// StartGenThumbnails re-triggers presets to (re)capture their thumbnails, one at a time per camera
// (so as not to fight over a single camera's position) but concurrently across cameras.
// allowLiveMove lets it move a camera that's currently live (normally blocked); includeExisting
// makes it redo presets that already have a thumbnail instead of only filling in missing ones.
// Starting a new run (or loading another config) cancels any run already in flight.
func (s *Store) StartGenThumbnails(allowLiveMove, includeExisting bool) {
	s.genMu.Lock()
	s.genToken++
	token := s.genToken
	s.genMu.Unlock()

	s.mu.Lock()
	cams := make([]*Camera, 0, len(s.cameras))
	for _, cam := range s.cameras {
		if !cam.IsHidden {
			cams = append(cams, cam)
		}
	}
	order := make(map[int][]string, len(cams))
	for _, cam := range cams {
		ids := genOrderLocked(cam)
		order[cam.CameraNum] = ids
		cam.Generating = len(ids) > 0
	}
	s.mu.Unlock()
	s.broadcast()

	for _, cam := range cams {
		ids := order[cam.CameraNum]
		if len(ids) == 0 {
			continue
		}
		go s.genCamera(cam.CameraNum, ids, token, allowLiveMove, includeExisting)
	}
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

func (s *Store) genCamera(cameraNum int, presetIDs []string, token int, allowLiveMove, includeExisting bool) {
	for _, presetID := range presetIDs {
		if s.genCancelled(token) {
			return
		}

		s.mu.Lock()
		cam := s.findCameraLocked(cameraNum)
		if cam == nil {
			s.mu.Unlock()
			return
		}
		preset := s.findPresetLocked(cam, presetID)
		if preset == nil {
			// preset was deleted mid-run
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
			s.broadcast()
			continue
		}

		// The camera never needs to (and, per isTriggerableLocked, isn't allowed to) "trigger" a
		// preset it's already sitting on — e.g. the preset that was active when the app booted.
		// Capture a snapshot directly instead of going through the normal move-and-settle flow.
		if alreadyActive {
			thumb, err := client.Snapshot()
			s.mu.Lock()
			if p := s.presetsByID[presetID]; p != nil && err == nil {
				p.Thumbnail = thumb
				p.ThumbnailVersion++
			}
			s.mu.Unlock()
			s.broadcast()
			continue
		}

		gen, _ := s.triggerPreset(cameraNum, presetID, allowLiveMove) // ignore error: e.g. already-triggering just means nothing to do

		settled := false
		deadline := time.Now().Add(genSettleTimeout)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			c := s.findCameraLocked(cameraNum)
			stillTriggering := c != nil && c.TriggeringPresetID != nil
			supersededByManualTrigger := c != nil && c.TriggerGen != gen
			s.mu.Unlock()
			if c == nil {
				return
			}
			if supersededByManualTrigger {
				s.mu.Lock()
				if cam := s.findCameraLocked(cameraNum); cam != nil {
					cam.Generating = false
				}
				s.mu.Unlock()
				s.broadcast()
				return
			}
			if !stillTriggering {
				settled = true
				break
			}
			time.Sleep(genPollInterval)
		}

		if !settled {
			s.mu.Lock()
			if cam := s.findCameraLocked(cameraNum); cam != nil {
				cam.Generating = false
			}
			s.mu.Unlock()
			s.broadcast()
			return
		}
		s.broadcast()
	}

	s.mu.Lock()
	if cam := s.findCameraLocked(cameraNum); cam != nil {
		cam.Generating = false
	}
	s.mu.Unlock()
	s.broadcast()
}
