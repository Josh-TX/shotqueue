package api

import (
	"encoding/json"
	"net/http"

	"shotqueue-backend/internal/settings"
	"shotqueue-backend/internal/state"
)

// publicCameraSettings strips the password before a CameraSettings goes out over the wire.
func publicCameraSettings(cs settings.CameraSettings) map[string]any {
	return map[string]any{
		"cameraNum": cs.CameraNum,
		"host":      cs.Host,
		"port":      cs.Port,
		"username":  cs.Username,
	}
}

func publicCamerasSettings(cams []settings.CameraSettings) []map[string]any {
	out := make([]map[string]any, len(cams))
	for i, cs := range cams {
		out[i] = publicCameraSettings(cs)
	}
	return out
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.settings.Get()
		writeJSON(w, 200, map[string]any{
			"atemHost": cfg.Atem.Host,
			"cameras":  publicCamerasSettings(cfg.Cameras),
			// Read-only; only here so the help modal can tell the user where configs live.
			"configPath": s.config.Path(),
		})

	case http.MethodPut:
		var body struct {
			AtemHost string `json:"atemHost"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := s.settings.SetAtem(settings.Atem{Host: body.AtemHost}); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if s.onAtemChange != nil {
			s.onAtemChange(body.AtemHost)
		}
		writeJSON(w, 200, map[string]any{"atemHost": body.AtemHost})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// POST /api/settings/cameras -> add a camera. Password can only be set here, at creation time.
func (s *Server) handleSettingsCameras(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		CameraNum int    `json:"cameraNum"`
		Host      string `json:"host"`
		Port      string `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if body.CameraNum <= 0 || body.Host == "" || body.Port == "" {
		writeError(w, 400, "cameraNum, host and port are required")
		return
	}
	cs := settings.CameraSettings{
		CameraNum: body.CameraNum,
		Host:      body.Host,
		Port:      body.Port,
		Username:  body.Username,
		Password:  body.Password,
	}
	if err := s.settings.AddCamera(cs); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	s.store.UpsertCamera(state.CameraSeed{
		CameraNum: cs.CameraNum, Host: cs.Host, Port: cs.Port, Username: cs.Username, Password: cs.Password,
	})
	writeJSON(w, 201, publicCameraSettings(cs))
}

// handleSettingsCameraSubroutes dispatches /api/settings/cameras/:cameraNum (edit host/port/username, delete).
func (s *Server) handleSettingsCameraSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := pathParts("/api/settings/cameras/", r.URL.Path)
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	num, ok := atoi(parts[0])
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Host     string `json:"host"`
			Port     string `json:"port"`
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if body.Host == "" || body.Port == "" {
			writeError(w, 400, "host and port are required")
			return
		}
		updated, err := s.settings.UpdateCamera(num, body.Host, body.Port, body.Username)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		// Rebuilds the ptz client in place; presets, groups and visibility are kept.
		s.store.UpsertCamera(state.CameraSeed{
			CameraNum: updated.CameraNum, Host: updated.Host, Port: updated.Port,
			Username: updated.Username, Password: updated.Password,
		})
		writeJSON(w, 200, publicCameraSettings(updated))

	case http.MethodDelete:
		if err := s.settings.DeleteCamera(num); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		s.store.RemoveCamera(num)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
