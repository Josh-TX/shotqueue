package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"shotqueue-backend/internal/api"
	"shotqueue-backend/internal/atem"
	"shotqueue-backend/internal/config"
	"shotqueue-backend/internal/state"
	"shotqueue-backend/internal/versions"
)

const autosaveInterval = 30 * time.Minute

// newAutosaveTimer runs a timer that saves an autosave version (if the state actually changed
// since the last one) every autosaveInterval. The returned func resets the timer early, used when
// a version is loaded so a load isn't immediately followed by a spurious autosave.
func newAutosaveTimer(store *state.Store, vstore *versions.Store) func() {
	reset := make(chan struct{}, 1)
	go func() {
		timer := time.NewTimer(autosaveInterval)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				vstore.MaybeAutosave(store.BuildSnapshot())
				timer.Reset(autosaveInterval)
			case <-reset:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(autosaveInterval)
			}
		}
	}()
	return func() {
		select {
		case reset <- struct{}{}:
		default:
		}
	}
}

// webDistDir resolves relative to this source file so `go run .` works from anywhere.
func webDistDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "frontend", "dist")
}

// atemSupervisor owns the currently-running atem.Listener and lets it be restarted when the ATEM
// host changes via the settings API.
type atemSupervisor struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	store  *state.Store
}

func (a *atemSupervisor) restart(host string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
	if host == "" {
		a.cancel = nil
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	listener := &atem.Listener{Host: host, OnChange: a.store.ApplyTally}
	go listener.Run(ctx)
}

func main() {
	port := flag.String("port", "8080", "port to serve the API and web UI on")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	store := state.New(cfg)
	supervisor := &atemSupervisor{store: store}

	versionsStore, err := versions.Load()
	if err != nil {
		log.Fatalf("loading versions: %v", err)
	}
	resetAutosave := newAutosaveTimer(store, versionsStore)

	server := api.New(store, cfg, versionsStore, supervisor.restart, resetAutosave)
	supervisor.restart(cfg.Get().Atem.Host)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	dist := webDistDir()
	if _, err := os.Stat(dist); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(dist)))
	} else {
		log.Printf("frontend/dist not found at %s; run `npm run build` in frontend/ to serve the UI", dist)
	}

	addr := ":" + *port
	log.Printf("shotqueue backend listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
