// Package api wires shotqueue's HTTP+websocket surface (ported from server/src/routes/api.js and
// index.js) onto internal/state and internal/settings.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"shotqueue-backend/internal/config"
	"shotqueue-backend/internal/settings"
	"shotqueue-backend/internal/state"
)

type Server struct {
	store        *state.Store
	settings     *settings.Store
	config       *config.Store
	onAtemChange func(host string)
	upgrader     websocket.Upgrader
	mu           sync.Mutex
	clients      map[*websocket.Conn]struct{}
}

// New wires the HTTP surface onto store/settings/config. onAtemChange is called after a settings
// update changes the ATEM host, so main can restart the tally listener against the new address.
func New(store *state.Store, settingsStore *settings.Store, configStore *config.Store, onAtemChange func(host string)) *Server {
	s := &Server{
		store:        store,
		settings:     settingsStore,
		config:       configStore,
		onAtemChange: onAtemChange,
		clients:      make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	store.SetBroadcaster(s.broadcastState)
	return s
}

// RegisterRoutes adds shotqueue's API/websocket routes to mux, leaving room for the caller to add
// a static file handler for "/" on the same mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ws", s.handleWS)

	mux.HandleFunc("/api/cameras", s.handleCameras)
	mux.HandleFunc("/api/cameras/", s.handleCameraSubroutes)
	mux.HandleFunc("/api/presets/", s.handlePresetThumbnail)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/configs", s.handleConfigs)
	mux.HandleFunc("/api/configs/", s.handleConfigSubroutes)
	mux.HandleFunc("/api/thumbnails/generate", s.handleGenThumbnails)
}

// ---- websocket ----

type stateMessage struct {
	Type          string            `json:"type"`
	Cameras       []state.CameraDTO `json:"cameras"`
	AtemConnected bool              `json:"atemConnected"`
}

func (s *Server) currentStateMessage() stateMessage {
	return stateMessage{Type: "state", Cameras: s.store.PublicCameras(), AtemConnected: s.store.AtemConnected()}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.clients[conn] = struct{}{}
	first := len(s.clients) == 1
	s.mu.Unlock()
	if first {
		// Only poll cameras for out-of-band position changes while someone's actually watching.
		s.store.Start()
	}

	payload, _ := json.Marshal(s.currentStateMessage())
	conn.WriteMessage(websocket.TextMessage, payload)

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.clients, conn)
			last := len(s.clients) == 0
			s.mu.Unlock()
			conn.Close()
			if last {
				s.store.Stop()
			}
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (s *Server) broadcastState() {
	// Marshal while holding the lock so concurrent broadcasts (e.g. one per camera finishing
	// thumbnail generation) can't race: without this, a stale snapshot marshaled earlier could
	// win the lock and send after a fresher one, leaving clients stuck on old state.
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, err := json.Marshal(s.currentStateMessage())
	if err != nil {
		return
	}
	for conn := range s.clients {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			conn.Close()
			delete(s.clients, conn)
		}
	}
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeLogicError(w http.ResponseWriter, err error) {
	if le, ok := err.(*state.LogicError); ok {
		writeError(w, le.Status, le.Message)
		return
	}
	log.Printf("[api] internal error: %v", err)
	writeError(w, 500, "internal error")
}

// pathParts splits "/api/cameras/3/presets/5/trigger" (after trimming "/api/") into segments.
func pathParts(prefix, path string) []string {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func (s *Server) handlePresetThumbnail(w http.ResponseWriter, r *http.Request) {
	parts := pathParts("/api/presets/", r.URL.Path)
	if len(parts) != 2 || parts[1] != "thumbnail" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	thumb, ok := s.store.PresetThumbnail(id)
	if !ok || thumb == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(thumb)
}

func nowMs() int64 { return time.Now().UnixMilli() }
