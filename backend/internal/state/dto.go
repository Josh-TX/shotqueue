package state

type PresetDTO struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ThumbnailVersion *int   `json:"thumbnailVersion"`
	WasTriggered     bool   `json:"wasTriggered"`
}

type CameraDTO struct {
	ID                      string      `json:"id"`
	IP                      string      `json:"ip"`
	Port                    string      `json:"port"`
	TallySource             uint16      `json:"tallySource"`
	Name                    string      `json:"name"`
	Status                  string      `json:"status"`
	Triggering              bool        `json:"triggering"`
	TriggeringPresetID      *string     `json:"triggeringPresetId"`
	SelectedGroupID         *int        `json:"selectedGroupId"`
	Queued                  *Queued     `json:"queued"`
	Presets                 []PresetDTO `json:"presets"`
	Groups                  []Group     `json:"groups"`
	ColumnCount             int         `json:"columnCount"`
	ActivePresetID          *string     `json:"activePresetId,omitempty"`
	Generating              bool        `json:"generating"`
}

func presetDTO(p *Preset) PresetDTO {
	var version *int
	if len(p.Thumbnail) > 0 {
		v := p.ThumbnailVersion
		version = &v
	}
	return PresetDTO{
		ID:               p.ID,
		Name:             p.Name,
		ThumbnailVersion: version,
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
		members := make([]string, len(g.Members))
		copy(members, g.Members)
		groups[i] = Group{ID: g.ID, Name: g.Name, Members: members}
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
		Generating:              cam.Generating,
	}
}

// PublicCamera returns a JSON-safe snapshot of one camera.
func (s *Store) PublicCamera(cameraID string) (CameraDTO, bool) {
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
