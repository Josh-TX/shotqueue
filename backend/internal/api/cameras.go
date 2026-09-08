package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"shotqueue-backend/internal/ptz"
)

// GET  /api/cameras       -> list
// POST /api/cameras       -> add
func (s *Server) handleCameras(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.store.PublicCameras())

	case http.MethodPost:
		var body struct {
			Name        string `json:"name"`
			Host        string `json:"host"`
			Port        string `json:"port"`
			TallySource uint16 `json:"tallySource"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if body.Host == "" || body.Port == "" {
			writeError(w, 400, "host and port are required")
			return
		}
		if body.Name == "" {
			body.Name = fmt.Sprintf("Camera %s", body.Host)
		}
		cam, err := s.store.AddCamera(body.Name, body.Host, body.Port, body.TallySource)
		if err != nil {
			writeLogicError(w, err)
			return
		}
		dto, _ := s.store.PublicCamera(cam.ID)
		writeJSON(w, 201, dto)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleCameraSubroutes dispatches everything under /api/cameras/... that isn't the collection
// route above: /api/cameras/test, /api/cameras/:id, and its nested preset/group/position routes.
func (s *Server) handleCameraSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := pathParts("/api/cameras/", r.URL.Path)
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}

	if parts[0] == "test" {
		if len(parts) == 2 && parts[1] == "snapshot" && r.Method == http.MethodGet {
			s.handleTestSnapshot(w, r, r.URL.Query().Get("host"), r.URL.Query().Get("port"))
			return
		}
		if len(parts) == 1 && r.Method == http.MethodPost {
			s.handleTestCamera(w, r)
			return
		}
		http.NotFound(w, r)
		return
	}

	camID := parts[0]
	rest := parts[1:]

	switch {
	case len(rest) == 0:
		s.handleCameraByID(w, r, camID)
	case len(rest) == 1 && rest[0] == "snapshot":
		s.handleSnapshot(w, r, camID)
	case len(rest) == 1 && rest[0] == "presets":
		s.handleAddPreset(w, r, camID)
	case len(rest) == 2 && rest[0] == "presets":
		s.handlePresetByID(w, r, camID, rest[1])
	case len(rest) == 3 && rest[0] == "presets" && rest[2] == "trigger":
		s.handleTriggerPreset(w, r, camID, rest[1])
	case len(rest) == 3 && rest[0] == "presets" && rest[2] == "queue":
		s.handleQueuePreset(w, r, camID, rest[1])
	case len(rest) == 1 && rest[0] == "queue":
		s.handleUnqueue(w, r, camID)
	case len(rest) == 1 && rest[0] == "selected-group":
		s.handleSelectedGroup(w, r, camID)
	case len(rest) == 1 && rest[0] == "group-count":
		s.handleGroupCount(w, r, camID)
	case len(rest) == 1 && rest[0] == "preset-order":
		s.handleReorderPresets(w, r, camID)
	case len(rest) == 2 && rest[0] == "groups":
		s.handleGroupByID(w, r, camID, rest[1])
	case len(rest) == 4 && rest[0] == "groups" && rest[2] == "members":
		s.handleGroupMember(w, r, camID, rest[1], rest[3])
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleTestCamera(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Host string `json:"host"`
		Port string `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Host == "" || body.Port == "" {
		writeError(w, 400, "host and port are required")
		return
	}
	client := ptz.New(body.Host, body.Port)
	pos, err := client.GetPosition()
	if err != nil {
		writeError(w, 502, fmt.Sprintf("no camera reachable at %s:%s: %v", body.Host, body.Port, err))
		return
	}
	writeJSON(w, 200, map[string]any{
		"ok":            true,
		"suggestedName": fmt.Sprintf("Camera %s", body.Host),
		"position":      pos,
		"snapshotUrl":   fmt.Sprintf("/api/cameras/test/snapshot?host=%s&port=%s&ts=%d", body.Host, body.Port, nowMs()),
	})
}

func (s *Server) handleCameraByID(w http.ResponseWriter, r *http.Request, camID string) {
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Name        string `json:"name"`
			Host        string `json:"host"`
			Port        string `json:"port"`
			TallySource uint16 `json:"tallySource"`
			ColumnCount int    `json:"columnCount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		cam := s.store.FindCamera(camID)
		if cam == nil {
			writeError(w, 404, "camera not found")
			return
		}
		if body.Name == "" {
			body.Name = cam.Name
		}
		if body.Host == "" {
			body.Host = cam.Host
		}
		if body.Port == "" {
			body.Port = cam.Port
		}
		if body.ColumnCount == 0 {
			body.ColumnCount = cam.ColumnCount
		}
		if err := s.store.UpdateCamera(camID, body.Name, body.Host, body.Port, body.TallySource, body.ColumnCount); err != nil {
			writeLogicError(w, err)
			return
		}
		dto, _ := s.store.PublicCamera(camID)
		writeJSON(w, 200, dto)

	case http.MethodDelete:
		if err := s.store.RemoveCamera(camID); err != nil {
			writeLogicError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request, camID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	client, ok := s.store.ClientFor(camID)
	if !ok {
		writeError(w, 404, "camera not found")
		return
	}
	img, err := client.Snapshot()
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(img)
}

// ---- settings (config passthrough for the test-snapshot route used before a camera is added) ----

func (s *Server) handleTestSnapshot(w http.ResponseWriter, r *http.Request, host, port string) {
	client := ptz.New(host, port)
	img, err := client.Snapshot()
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(img)
}
