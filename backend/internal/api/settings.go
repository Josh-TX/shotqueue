package api

import (
	"encoding/json"
	"net/http"

	"shotqueue-backend/internal/settings"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]any{"atemHost": s.settings.Get().Atem.Host})

	case http.MethodPut:
		var body struct {
			AtemHost string `json:"atemHost"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := s.settings.SetAtem(settings.Atem{Host: body.AtemHost}); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if s.onAtemChange != nil {
			s.onAtemChange(body.AtemHost)
		}
		writeJSON(w, 200, map[string]any{"atemHost": body.AtemHost})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
