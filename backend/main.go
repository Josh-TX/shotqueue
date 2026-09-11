package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"sync"

	"shotqueue-backend/internal/api"
	"shotqueue-backend/internal/atem"
	"shotqueue-backend/internal/config"
	"shotqueue-backend/internal/settings"
	"shotqueue-backend/internal/state"
)

// webdist holds the built frontend, copied here by build scripts/CI before
// compiling so the resulting binary is self-contained. See webdist/placeholder.
//
//go:embed webdist
var webdistFS embed.FS

// frontendHandler returns nil if no frontend has been embedded (e.g. local
// `go run .` without running the frontend build first), so callers can fall
// back to running the Vite dev server separately.
func frontendHandler() http.Handler {
	sub, err := fs.Sub(webdistFS, "webdist")
	if err != nil {
		log.Fatalf("reading embedded webdist: %v", err)
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return http.FileServer(http.FS(sub))
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
		seeds, dropped := state.BuildLoadSeeds(latest.Cameras, settingsStore)
		if len(dropped) > 0 {
			log.Printf("dropped cameraNums %v from latest config (no longer in settings)", dropped)
		}
		if err := store.LoadConfig(seeds); err != nil {
			log.Fatalf("loading latest config: %v", err)
		}
	}
	store.SetOnMutate(func() { configStore.Autosave(store.BuildSnapshot()) })

	supervisor := &atemSupervisor{store: store}

	server := api.New(store, settingsStore, configStore, supervisor.restart)
	supervisor.restart(settingsStore.Get().Atem.Host)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	if handler := frontendHandler(); handler != nil {
		mux.Handle("/", handler)
	} else {
		log.Printf("no frontend embedded; run `npm run build` in frontend/ and copy dist/ into backend/webdist, or run the Vite dev server separately")
	}

	addr := ":" + *port
	log.Printf("shotqueue backend listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
