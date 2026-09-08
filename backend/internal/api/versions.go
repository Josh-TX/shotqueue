package api

import (
	"encoding/json"
	"net/http"
)

// GET  /api/versions -> list every version (named + autosave), full snapshot content included
// POST /api/versions -> save a named version; overwrites by case-insensitive name match
func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.versions.List())

	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, 400, "name is required")
			return
		}
		snapshot := s.store.BuildSnapshot()
		v, err := s.versions.SaveNamed(body.Name, snapshot)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, v)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleVersionSubroutes dispatches /api/versions/:id (delete) and /api/versions/:id/load.
func (s *Server) handleVersionSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := pathParts("/api/versions/", r.URL.Path)
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	rest := parts[1:]

	switch {
	case len(rest) == 0:
		s.handleVersionByID(w, r, id)
	case len(rest) == 1 && rest[0] == "load":
		s.handleLoadVersion(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleVersionByID(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.versions.DeleteNamed(id); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLoadVersion(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	v, ok := s.versions.Get(id)
	if !ok {
		writeError(w, 404, "version not found")
		return
	}
	var body struct {
		GenerateThumbnails bool `json:"generateThumbnails"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if err := s.store.LoadVersion(v.Cameras); err != nil {
		writeLogicError(w, err)
		return
	}
	if body.GenerateThumbnails {
		s.store.StartGenThumbnails(false, true)
	}
	writeJSON(w, 200, s.store.PublicCameras())
}
