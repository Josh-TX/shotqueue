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

	"shotqueue-backend/internal/api"
	"shotqueue-backend/internal/atem"
	"shotqueue-backend/internal/config"
	"shotqueue-backend/internal/settings"
	"shotqueue-backend/internal/state"
)

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
		a.store.SetAtemConnected(false)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	listener := &atem.Listener{
		Host:         host,
		OnChange:     a.store.ApplyTally,
		OnConnect:    func() { a.store.SetAtemConnected(true) },
		OnDisconnect: func() { a.store.SetAtemConnected(false) },
	}
	go listener.Run(ctx)
}

func main() {
	port := flag.String("port", "8080", "port to serve the API and web UI on")
	flag.Parse()

	settingsStore, err := settings.Load()
	if err != nil {
		log.Fatalf("loading settings: %v", err)
	}

	configStore, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	store := state.New()
	if latest, ok := configStore.LatestAutosave(); ok {
		if err := store.LoadConfig(latest.Cameras); err != nil {
			log.Fatalf("loading latest config: %v", err)
		}
	}
	store.SetOnMutate(func() { configStore.Autosave(store.BuildSnapshot()) })

	supervisor := &atemSupervisor{store: store}

	server := api.New(store, settingsStore, configStore, supervisor.restart)
	supervisor.restart(settingsStore.Get().Atem.Host)

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
