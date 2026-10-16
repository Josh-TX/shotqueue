package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"shotqueue-backend/internal/state"
)

// GET  /api/scenes -> list
// POST /api/scenes -> add a scene
func (s *Server) handleScenes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.store.PublicScenes())
	case http.MethodPost:
		writeJSON(w, 200, s.store.AddScene())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// PATCH /api/scene-order {order: [sceneId...]}
func (s *Server) handleSceneOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Order []string `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if err := s.store.ReorderScenes(body.Order); err != nil {
		writeLogicError(w, err)
		return
	}
	writeJSON(w, 200, s.store.PublicScenes())
}

// handleSceneSubroutes dispatches /api/scenes/:id[/execute|/duplicate|/cameras/:cameraNum].
func (s *Server) handleSceneSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := pathParts("/api/scenes/", r.URL.Path)
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	rest := parts[1:]
	switch {
	case len(rest) == 0:
		s.handleSceneByID(w, r, id)
	case len(rest) == 1 && rest[0] == "execute":
		s.handleExecuteScene(w, r, id)
	case len(rest) == 1 && rest[0] == "duplicate":
		s.handleDuplicateScene(w, r, id)
	case len(rest) == 2 && rest[0] == "cameras":
		camNum, ok := atoi(rest[1])
		if !ok {
			http.NotFound(w, r)
			return
		}
		s.handleSceneCamera(w, r, id, camNum)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleSceneByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := s.store.RenameScene(id, strings.TrimSpace(body.Name)); err != nil {
			writeLogicError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := s.store.DeleteScene(id); err != nil {
			writeLogicError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleExecuteScene(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.ExecuteScene(id); err != nil {
		writeLogicError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDuplicateScene(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	dto, err := s.store.DuplicateScene(id)
	if err != nil {
		writeLogicError(w, err)
		return
	}
	writeJSON(w, 200, dto)
}

// PATCH /api/scenes/:id/cameras/:cameraNum {isHidden?, groupId?, triggerPresetId?, queuePresetId?}
// Keys that are present are applied; for the nullable ones, null clears.
func (s *Server) handleSceneCamera(w http.ResponseWriter, r *http.Request, id string, camNum int) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		IsHidden        *bool           `json:"isHidden"`
		GroupID         json.RawMessage `json:"groupId"`
		GroupIDs        []int           `json:"groupIds"`
		GroupMode       string          `json:"groupMode"`
		TriggerPresetID json.RawMessage `json:"triggerPresetId"`
		QueuePresetID   json.RawMessage `json:"queuePresetId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	patch := state.SceneCameraPatch{IsHidden: body.IsHidden}
	if body.GroupID != nil {
		patch.SetGroup = true
		patch.GroupIDs = body.GroupIDs
		patch.GroupMode = body.GroupMode
		if err := json.Unmarshal(body.GroupID, &patch.GroupID); err != nil {
			writeError(w, 400, "invalid groupId")
			return
		}
	}
	if body.TriggerPresetID != nil {
		patch.SetTriggerPresetID = true
		if err := json.Unmarshal(body.TriggerPresetID, &patch.TriggerPresetID); err != nil {
			writeError(w, 400, "invalid triggerPresetId")
			return
		}
	}
	if body.QueuePresetID != nil {
		patch.SetQueuePresetID = true
		if err := json.Unmarshal(body.QueuePresetID, &patch.QueuePresetID); err != nil {
			writeError(w, 400, "invalid queuePresetId")
			return
		}
	}
	if err := s.store.UpdateSceneCamera(id, camNum, patch); err != nil {
		writeLogicError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
