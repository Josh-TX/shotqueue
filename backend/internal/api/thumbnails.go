package api

import (
	"encoding/json"
	"net/http"
)

// POST /api/thumbnails/generate -> kick off a thumbnail generation run across every camera.
// Progress is reported over the websocket via the normal state broadcast (generating per camera).
func (s *Server) handleGenThumbnails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		AllowLiveMove   bool `json:"allowLiveMove"`
		IncludeExisting bool `json:"includeExisting"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	s.store.StartGenThumbnails(body.AllowLiveMove, body.IncludeExisting)
	writeJSON(w, 202, s.store.PublicCameras())
}
