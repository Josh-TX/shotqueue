package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"shotqueue-backend/internal/ptz"
)

// GET /api/cameras -> list
func (s *Server) handleCameras(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, 200, s.store.PublicCameras())
}

// handleCameraSubroutes dispatches everything under /api/cameras/... that isn't the collection
// route above: /api/cameras/test, /api/cameras/:cameraNum, and its nested preset/group/position
// routes.
func (s *Server) handleCameraSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := pathParts("/api/cameras/", r.URL.Path)
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}

	if parts[0] == "test" {
		if len(parts) == 2 && parts[1] == "snapshot" && r.Method == http.MethodGet {
			q := r.URL.Query()
			s.handleTestSnapshot(w, r, q.Get("host"), q.Get("port"), q.Get("username"), q.Get("password"))
			return
		}
		if len(parts) == 1 && r.Method == http.MethodPost {
			s.handleTestCamera(w, r)
			return
		}
		http.NotFound(w, r)
		return
	}

	camNum, ok := atoi(parts[0])
	if !ok {
		http.NotFound(w, r)
		return
	}
	rest := parts[1:]

	switch {
	case len(rest) == 0:
		s.handleCameraByID(w, r, camNum)
	case len(rest) == 1 && rest[0] == "snapshot":
		s.handleSnapshot(w, r, camNum)
	case len(rest) == 1 && rest[0] == "presets":
		s.handleAddPreset(w, r, camNum)
	case len(rest) == 2 && rest[0] == "presets":
		s.handlePresetByID(w, r, camNum, rest[1])
	case len(rest) == 3 && rest[0] == "presets" && rest[2] == "trigger":
		s.handleTriggerPreset(w, r, camNum, rest[1])
	case len(rest) == 3 && rest[0] == "presets" && rest[2] == "queue":
		s.handleQueuePreset(w, r, camNum, rest[1])
	case len(rest) == 3 && rest[0] == "presets" && rest[2] == "position":
		s.handleUpdatePresetPosition(w, r, camNum, rest[1])
	case len(rest) == 1 && rest[0] == "queue":
		s.handleUnqueue(w, r, camNum)
	case len(rest) == 1 && rest[0] == "selected-group":
		s.handleSelectedGroup(w, r, camNum)
	case len(rest) == 1 && rest[0] == "group-count":
		s.handleGroupCount(w, r, camNum)
	case len(rest) == 1 && rest[0] == "preset-order":
		s.handleReorderPresets(w, r, camNum)
	case len(rest) == 2 && rest[0] == "groups":
		s.handleGroupByID(w, r, camNum, rest[1])
	case len(rest) == 4 && rest[0] == "groups" && rest[2] == "members":
		s.handleGroupMember(w, r, camNum, rest[1], rest[3])
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleTestCamera(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Host == "" || body.Port == "" {
		writeError(w, 400, "host and port are required")
		return
	}
	client := ptz.New(body.Host, body.Port, body.Username, body.Password)
	pos, err := client.GetPosition()
	if err != nil {
		writeError(w, 502, fmt.Sprintf("no camera reachable at %s:%s: %v", body.Host, body.Port, err))
		return
	}
	writeJSON(w, 200, map[string]any{
		"ok":       true,
		"position": pos,
		"snapshotUrl": fmt.Sprintf("/api/cameras/test/snapshot?host=%s&port=%s&username=%s&password=%s&ts=%d",
			url.QueryEscape(body.Host), url.QueryEscape(body.Port), url.QueryEscape(body.Username), url.QueryEscape(body.Password), nowMs()),
	})
}

func (s *Server) handleCameraByID(w http.ResponseWriter, r *http.Request, camNum int) {
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			ColumnCount int `json:"columnCount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		cam := s.store.FindCamera(camNum)
		if cam == nil {
			writeError(w, 404, "camera not found")
			return
		}
		if body.ColumnCount == 0 {
			body.ColumnCount = cam.ColumnCount
		}
		if err := s.store.SetColumnCount(camNum, body.ColumnCount); err != nil {
			writeLogicError(w, err)
			return
		}
		dto, _ := s.store.PublicCamera(camNum)
		writeJSON(w, 200, dto)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request, camNum int) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	client, ok := s.store.ClientFor(camNum)
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

func (s *Server) handleTestSnapshot(w http.ResponseWriter, r *http.Request, host, port, username, password string) {
	client := ptz.New(host, port, username, password)
	img, err := client.Snapshot()
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(img)
}
