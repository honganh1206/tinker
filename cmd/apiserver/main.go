package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/honganh1206/tinker/internal/apiserver"
	"github.com/honganh1206/tinker/internal/eventbus"
	"github.com/honganh1206/tinker/internal/logger"
	"github.com/honganh1206/tinker/internal/web"
)

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid .env line: %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func main() {
	if err := loadDotEnv(".env"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load .env: %v\n", err)
		os.Exit(1)
	}

	var addr string
	var eventBusURL string
	var sessionDir string
	var googleClientID string
	var googleClientSecret string
	var googleRedirectURL string

	flag.StringVar(&addr, "addr", ":11435", "Listen address")
	flag.StringVar(&eventBusURL, "event-bus-url", os.Getenv("NATS_LOCAL_PORT"), "NATS event bus URL")
	flag.StringVar(&sessionDir, "store-dir", "", "Session store directory (default ~/.tinker/sessions)")
	flag.StringVar(&googleClientID, "google-client-id", os.Getenv("GOOGLE_CLIENT_ID"), "Google OAuth client ID (empty disables auth)")
	flag.StringVar(&googleClientSecret, "google-client-secret", os.Getenv("GOOGLE_CLIENT_SECRET"), "Google OAuth client secret")
	flag.StringVar(&googleRedirectURL, "google-redirect-url", os.Getenv("GOOGLE_REDIRECT_URL"), "Google OAuth redirect URL")
	flag.Parse()

	log := logger.NewLogger(os.Stderr, true)

	if googleClientID != "" && (googleClientSecret == "" || googleRedirectURL == "") {
		log.Error("--google-client-secret and --google-redirect-url are required when --google-client-id is set")
		os.Exit(1)
	}

	authConfig := apiserver.AuthConfig{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  googleRedirectURL,
	}

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

	srv := apiserver.NewServer(bus, log, sessionDir, mcpDir, authConfig)

	if err := srv.Start(addr, frontendFS); err != nil {
		log.Error("api server failed", "error", err)
		os.Exit(1)
	}
}
