package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"

	"shotqueue-backend/internal/api"
	"shotqueue-backend/internal/atem"
	"shotqueue-backend/internal/cli"
	"shotqueue-backend/internal/config"
	"shotqueue-backend/internal/settings"
	"shotqueue-backend/internal/state"
	"shotqueue-backend/internal/webui"
)

var version = "dev" // set via -ldflags "-X main.version=..."

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
	opts, err := cli.Load(os.Args[1:], version)
	if err != nil {
		log.Fatal(err)
	}

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
		if err := store.LoadConfig(seeds, latest.Scenes); err != nil {
			log.Fatalf("loading latest config: %v", err)
		}
	}
	store.SetOnMutate(func() {
		cams, scenes := store.BuildSnapshot()
		configStore.Autosave(cams, scenes)
	})

	supervisor := &atemSupervisor{store: store}

	server := api.New(store, settingsStore, configStore, supervisor.restart)
	supervisor.restart(settingsStore.Get().Atem.Host)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	if webui.Embedded() {
		dist, err := webui.FS()
		if err != nil {
			log.Fatalf("reading embedded frontend: %v", err)
		}
		mux.Handle("/", http.FileServer(http.FS(dist)))
	} else {
		log.Printf("no frontend embedded, use the Vite dev server (or build with -tags embed_frontend)")
	}

	addr := ":" + opts.Port
	log.Printf("shotqueue backend listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
