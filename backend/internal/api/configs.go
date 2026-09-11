package api

import (
	"encoding/json"
	"log"
	"net/http"

	"shotqueue-backend/internal/state"
)

// GET  /api/configs -> list every config (named + autosave), full snapshot content included
// POST /api/configs -> save a named config; overwrites by case-insensitive name match
func (s *Server) handleConfigs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.config.List())

	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, 400, "name is required")
			return
		}
		snapshot := s.store.BuildSnapshot()
		c, err := s.config.SaveNamed(body.Name, snapshot)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, c)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleConfigSubroutes dispatches /api/configs/:id (delete) and /api/configs/:id/load.
func (s *Server) handleConfigSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := pathParts("/api/configs/", r.URL.Path)
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	rest := parts[1:]

	switch {
	case len(rest) == 0:
		s.handleConfigByID(w, r, id)
	case len(rest) == 1 && rest[0] == "load":
		s.handleLoadConfig(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleConfigByID(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.config.DeleteNamed(id); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLoadConfig(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	c, ok := s.config.Get(id)
	if !ok {
		writeError(w, 404, "config not found")
		return
	}
	var body struct {
		GenerateThumbnails bool `json:"generateThumbnails"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	seeds, dropped := state.BuildLoadSeeds(c.Cameras, s.settings)
	if len(dropped) > 0 {
		log.Printf("[api] load config %s: dropped cameraNums %v (no longer in settings)", id, dropped)
	}
	if err := s.store.LoadConfig(seeds); err != nil {
		writeLogicError(w, err)
		return
	}
	if body.GenerateThumbnails {
		s.store.StartGenThumbnails(false, true)
	}
	writeJSON(w, 200, s.store.PublicCameras())
}
