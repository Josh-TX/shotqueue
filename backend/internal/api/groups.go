package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleGroupCount(w http.ResponseWriter, r *http.Request, camID string) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if err := s.store.SetGroupCount(camID, body.Count); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID)
	writeJSON(w, 200, dto)
}

func (s *Server) handleGroupByID(w http.ResponseWriter, r *http.Request, camID string, groupIDStr string) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	groupID, ok := atoi(groupIDStr)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Name       *string `json:"name"`
		IsSequence *bool   `json:"isSequence"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if err := s.store.UpdateGroup(camID, groupID, body.Name, body.IsSequence); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID)
	writeJSON(w, 200, dto)
}

func (s *Server) handleGroupMember(w http.ResponseWriter, r *http.Request, camID string, groupIDStr, presetIDStr string) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	groupID, ok := atoi(groupIDStr)
	if !ok {
		http.NotFound(w, r)
		return
	}
	presetID := presetIDStr
	var body struct {
		InGroup bool `json:"inGroup"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if err := s.store.SetGroupMember(camID, groupID, presetID, body.InGroup); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID)
	writeJSON(w, 200, dto)
}
