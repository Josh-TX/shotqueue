// Package api wires shotqueue's HTTP+websocket surface (ported from server/src/routes/api.js and
// index.js) onto internal/state and internal/config.
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
	"shotqueue-backend/internal/state"
	"shotqueue-backend/internal/versions"
)

type Server struct {
	store         *state.Store
	cfg           *config.Store
	versions      *versions.Store
	onAtemChange  func(host string)
	onVersionLoad func()
	upgrader      websocket.Upgrader
	mu            sync.Mutex
	clients       map[*websocket.Conn]struct{}
}

// New wires the HTTP surface onto store/cfg/versions. onAtemChange is called after a settings
// update changes the ATEM host, so main can restart the tally listener against the new address.
// onVersionLoad is called after a version is loaded, so main can reset the autosave timer.
func New(store *state.Store, cfg *config.Store, versionsStore *versions.Store, onAtemChange func(host string), onVersionLoad func()) *Server {
	s := &Server{
		store:         store,
		cfg:           cfg,
		versions:      versionsStore,
		onAtemChange:  onAtemChange,
		onVersionLoad: onVersionLoad,
		clients:       make(map[*websocket.Conn]struct{}),
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
	mux.HandleFunc("/api/colors", s.handleColors)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/reset-show", s.handleResetShow)
	mux.HandleFunc("/api/reset-scene", s.handleResetScene)
	mux.HandleFunc("/api/versions", s.handleVersions)
	mux.HandleFunc("/api/versions/", s.handleVersionSubroutes)
}

// ---- websocket ----

type stateMessage struct {
	Type    string            `json:"type"`
	Cameras []state.CameraDTO `json:"cameras"`
}

func (s *Server) currentStateMessage() stateMessage {
	// Position/activePresetId is intentionally omitted here: the frontend only learns it via REST
	// polling of /position, never over the websocket.
	return stateMessage{Type: "state", Cameras: s.store.PublicCameras(false)}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	payload, _ := json.Marshal(s.currentStateMessage())
	conn.WriteMessage(websocket.TextMessage, payload)

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.clients, conn)
			s.mu.Unlock()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (s *Server) broadcastState() {
	payload, err := json.Marshal(s.currentStateMessage())
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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

// ---- misc top-level routes ----

func (s *Server) handleColors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, state.GroupColors)
}

func (s *Server) handleResetShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.store.ResetShowMetrics()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleResetScene(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.store.ResetSceneMetrics()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePresetThumbnail(w http.ResponseWriter, r *http.Request) {
	parts := pathParts("/api/presets/", r.URL.Path)
	if len(parts) != 2 || parts[1] != "thumbnail" {
		http.NotFound(w, r)
		return
	}
	id, ok := atoi(parts[0])
	if !ok {
		http.NotFound(w, r)
		return
	}
	thumb, ok := s.store.PresetThumbnail(id)
	if !ok || thumb == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(thumb)
}

func nowMs() int64 { return time.Now().UnixMilli() }
