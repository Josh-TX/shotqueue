package state

import "fmt"

type PresetDTO struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	ThumbnailURL     string `json:"thumbnailUrl"`
	ThumbnailVersion int    `json:"thumbnailVersion"`
	WasTriggered     bool   `json:"wasTriggered"`
}

type CameraDTO struct {
	ID                      int         `json:"id"`
	IP                      string      `json:"ip"`
	Port                    string      `json:"port"`
	TallySource             uint16      `json:"tallySource"`
	Name                    string      `json:"name"`
	Status                  string      `json:"status"`
	Triggering              bool        `json:"triggering"`
	TriggeringPresetID      int         `json:"triggeringPresetId"`
	SelectedGroupID         *int        `json:"selectedGroupId"`
	Queued                  *Queued     `json:"queued"`
	Presets                 []PresetDTO `json:"presets"`
	Groups                  []Group     `json:"groups"`
	ColumnCount             int         `json:"columnCount"`
	ActivePresetID          *int        `json:"activePresetId,omitempty"`
	CurrentThumbnailVersion int         `json:"currentThumbnailVersion"`
	Regenerating            bool        `json:"regenerating,omitempty"`
	RegenDone               int         `json:"regenDone,omitempty"`
	RegenTotal              int         `json:"regenTotal,omitempty"`
}

func presetDTO(p *Preset) PresetDTO {
	return PresetDTO{
		ID:               p.ID,
		Name:             p.Name,
		ThumbnailURL:     fmt.Sprintf("/api/presets/%d/thumbnail", p.ID),
		ThumbnailVersion: p.ThumbnailVersion,
		WasTriggered:     p.WasTriggered,
	}
}

// cameraDTOLocked builds a CameraDTO; caller must hold s.mu.
func cameraDTOLocked(cam *Camera) CameraDTO {
	presets := make([]PresetDTO, len(cam.Presets))
	for i, p := range cam.Presets {
		presets[i] = presetDTO(p)
	}
	groups := make([]Group, len(cam.Groups))
	for i, g := range cam.Groups {
		members := make([]int, len(g.Members))
		copy(members, g.Members)
		groups[i] = Group{ID: g.ID, Name: g.Name, Color: g.Color, Members: members}
	}
	return CameraDTO{
		ID:                      cam.ID,
		IP:                      cam.Host,
		Port:                    cam.Port,
		TallySource:             cam.TallySource,
		Name:                    cam.Name,
		Status:                  cam.Status,
		Triggering:              cam.Triggering,
		TriggeringPresetID:      cam.TriggeringPresetID,
		SelectedGroupID:         cam.SelectedGroupID,
		Queued:                  cam.Queued,
		Presets:                 presets,
		Groups:                  groups,
		ColumnCount:             cam.ColumnCount,
		ActivePresetID:          activePresetIDLocked(cam),
		CurrentThumbnailVersion: cam.CurrentThumbnailVersion,
		Regenerating:            cam.Regenerating,
		RegenDone:               cam.RegenDone,
		RegenTotal:              cam.RegenTotal,
	}
}

// PublicCamera returns a JSON-safe snapshot of one camera.
func (s *Store) PublicCamera(cameraID int) (CameraDTO, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam := s.findCameraLocked(cameraID)
	if cam == nil {
		return CameraDTO{}, false
	}
	return cameraDTOLocked(cam), true
}

// PublicCameras returns a JSON-safe snapshot of every camera, for the cameras list and the
// websocket structural broadcast.
func (s *Store) PublicCameras() []CameraDTO {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CameraDTO, len(s.cameras))
	for i, cam := range s.cameras {
		out[i] = cameraDTOLocked(cam)
	}
	return out
}
