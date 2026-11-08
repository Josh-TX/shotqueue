package api

import (
	"encoding/json"
	"net/http"

	"shotqueue-backend/internal/state"
)

func (s *Server) handleAddPreset(w http.ResponseWriter, r *http.Request, camNum int) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Name      string `json:"name"`
		GroupIDs  []int  `json:"groupIds"`
		SaveFocus bool   `json:"saveFocus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	_, warning, err := s.store.AddPreset(camNum, body.Name, body.GroupIDs, body.SaveFocus)
	if err != nil {
		writeLogicError(w, err)
		return
	}
	setWarning(w, warning)
	dto, _ := s.store.PublicCamera(camNum)
	writeJSON(w, 201, dto)
}

func (s *Server) handlePresetByID(w http.ResponseWriter, r *http.Request, camNum int, presetID string) {
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := s.store.RenamePreset(camNum, presetID, body.Name); err != nil {
			writeLogicError(w, err)
			return
		}
		dto, _ := s.store.PublicCamera(camNum)
		writeJSON(w, 200, dto)

	case http.MethodDelete:
		if err := s.store.DeletePreset(camNum, presetID); err != nil {
			writeLogicError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUpdatePresetPosition(w http.ResponseWriter, r *http.Request, camNum int, presetID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		SaveFocus bool `json:"saveFocus"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	warning, err := s.store.UpdatePresetPosition(camNum, presetID, body.SaveFocus)
	if err != nil {
		writeLogicError(w, err)
		return
	}
	setWarning(w, warning)
	dto, _ := s.store.PublicCamera(camNum)
	writeJSON(w, 200, dto)
}

func (s *Server) handleReorderPresets(w http.ResponseWriter, r *http.Request, camNum int) {
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
	if err := s.store.ReorderPresets(camNum, body.Order); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camNum)
	writeJSON(w, 200, dto)
}

func (s *Server) handleTriggerPreset(w http.ResponseWriter, r *http.Request, camNum int, presetID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.TriggerPreset(camNum, presetID); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camNum)
	writeJSON(w, 200, dto)
}

func (s *Server) handleQueuePreset(w http.ResponseWriter, r *http.Request, camNum int, presetID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.QueuePreset(camNum, presetID, "manual"); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camNum)
	writeJSON(w, 200, dto)
}

func (s *Server) handleUnqueue(w http.ResponseWriter, r *http.Request, camNum int) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.UnqueuePreset(camNum); err != nil {
		writeLogicError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSelectedGroup(w http.ResponseWriter, r *http.Request, camNum int) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		GroupID  *int   `json:"groupId"`
		GroupIDs []int  `json:"groupIds"`
		Mode     string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if err := s.store.SetAutoQueue(camNum, state.AutoSel{GroupID: body.GroupID, GroupIDs: body.GroupIDs, Mode: body.Mode}); err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camNum)
	writeJSON(w, 200, dto)
}

// setWarning passes a non-fatal message to the client, which shows it as a toast.
func setWarning(w http.ResponseWriter, warning string) {
	if warning != "" {
		w.Header().Set("X-Shotqueue-Warning", warning)
	}
}

// handleFocus: POST runs a one-touch focus on the active preset (and saves the result), DELETE
// forgets the preset's stored focus.
func (s *Server) handleFocus(w http.ResponseWriter, r *http.Request, camNum int, presetID string) {
	var err error
	switch r.Method {
	case http.MethodPost:
		err = s.store.OneTouchFocus(camNum, presetID)
	case http.MethodDelete:
		err = s.store.ForgetFocus(camNum, presetID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		writeLogicError(w, err)
		return
	}
	dto, _ := s.store.PublicCamera(camNum)
	writeJSON(w, 200, dto)
}
