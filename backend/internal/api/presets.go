package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleAddPreset(w http.ResponseWriter, r *http.Request, camID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Name     string `json:"name"`
		GroupIDs []int  `json:"groupIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if _, err := s.store.AddPreset(camID, body.Name, body.GroupIDs); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID)
	writeJSON(w, 201, dto)
}

func (s *Server) handlePresetByID(w http.ResponseWriter, r *http.Request, camID string, presetID string) {
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := s.store.RenamePreset(camID, presetID, body.Name); err != nil {
			writeLogicError(w, err)
			return
		}
		dto, _ := s.store.PublicCamera(camID)
		writeJSON(w, 200, dto)

	case http.MethodDelete:
		if err := s.store.DeletePreset(camID, presetID); err != nil {
			writeLogicError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleReorderPresets(w http.ResponseWriter, r *http.Request, camID string) {
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
	if err := s.store.ReorderPresets(camID, body.Order); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID)
	writeJSON(w, 200, dto)
}

func (s *Server) handleTriggerPreset(w http.ResponseWriter, r *http.Request, camID string, presetID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.TriggerPreset(camID, presetID); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID)
	writeJSON(w, 200, dto)
}

func (s *Server) handleQueuePreset(w http.ResponseWriter, r *http.Request, camID string, presetID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.QueuePreset(camID, presetID, "manual"); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID)
	writeJSON(w, 200, dto)
}

func (s *Server) handleUnqueue(w http.ResponseWriter, r *http.Request, camID string) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.UnqueuePreset(camID); err != nil {
		writeLogicError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSelectedGroup(w http.ResponseWriter, r *http.Request, camID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		GroupID *int `json:"groupId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if err := s.store.SetSelectedGroup(camID, body.GroupID); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID)
	writeJSON(w, 200, dto)
}
