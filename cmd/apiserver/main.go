package main

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/honganh1206/tinker/apiserver"
	"github.com/honganh1206/tinker/eventbus"
	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/web"
)

func main() {
	var addr string
	var eventBusURL string
	var sessionDir string

	flag.StringVar(&addr, "addr", ":11435", "Listen address")
	flag.StringVar(&eventBusURL, "event-bus-url", os.Getenv("NATS_LOCAL_PORT"), "NATS event bus URL")
	flag.StringVar(&sessionDir, "store-dir", "", "Session store directory (default ~/.tinker/sessions)")
	flag.Parse()

	log := logger.NewLogger(os.Stderr, true)

	if sessionDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Error("failed to get home directory", "error", err)
			os.Exit(1)
		}
		sessionDir = filepath.Join(home, ".tinker", "sessions")
	}

	mcpDir := ""
	home, err := os.UserHomeDir()
	if err == nil {
		mcpDir = filepath.Join(home, ".tinker", "mcp", "servers")
	}

	frontendFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Error("failed to load frontend assets", "error", err)
		os.Exit(1)
	}

	var bus eventbus.EventBus
	natsbus, err := eventbus.NewNATSEventBus(eventBusURL)
	if err != nil {
		log.Error("failed to connect to event bus, starting without streaming support", "error", err)
	} else {
		bus = natsbus
	}

	srv := apiserver.NewServer(bus, log, sessionDir, mcpDir)

	if err := srv.Start(addr, frontendFS); err != nil {
		log.Error("api server failed", "error", err)
		os.Exit(1)
	}
}
