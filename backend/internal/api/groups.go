package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleAddGroup(w http.ResponseWriter, r *http.Request, camID int) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if _, err := s.store.AddGroup(camID, body.Name); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID, true)
	writeJSON(w, 201, dto)
}

func (s *Server) handleGroupByID(w http.ResponseWriter, r *http.Request, camID int, groupIDStr string) {
	groupID, ok := atoi(groupIDStr)
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if err := s.store.UpdateGroup(camID, groupID, body.Name, body.Color); err != nil {
			writeLogicError(w, err)
			return
		}
		dto, _ := s.store.PublicCamera(camID, true)
		writeJSON(w, 200, dto)

	case http.MethodDelete:
		if err := s.store.DeleteGroup(camID, groupID); err != nil {
			writeLogicError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGroupMember(w http.ResponseWriter, r *http.Request, camID int, groupIDStr, presetIDStr string) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	groupID, ok1 := atoi(groupIDStr)
	presetID, ok2 := atoi(presetIDStr)
	if !ok1 || !ok2 {
		http.NotFound(w, r)
		return
	}
	var body struct {
		InGroup *bool `json:"inGroup"`
		Weight  *int  `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if err := s.store.SetGroupMember(camID, groupID, presetID, body.InGroup, body.Weight); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camID, true)
	writeJSON(w, 200, dto)
}
