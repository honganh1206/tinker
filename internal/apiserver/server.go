package apiserver

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/honganh1206/tinker/eventbus"
	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/storage"
)

// upgrader upgrades HTTP requests to WebSocket protocol via a handshake
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server is the Tinker API Server.
type Server struct {
	eventbus    eventbus.EventBus
	log         *logger.Logger
	sessionsDir string
	mcpStore    mcp.ConfigStore
	mux         *http.ServeMux

	clientsMu sync.Mutex
	clients   map[chan []byte]struct{}
}

// NewServer creates a new Tinker API server.
func NewServer(bus eventbus.EventBus, log *logger.Logger, sessionsDir, mcpDir string) *Server {
	s := &Server{
		eventbus:    bus,
		log:         log,
		sessionsDir: sessionsDir,
		mcpStore:    mcp.NewFileConfigStore(mcpDir),
		clients:     make(map[chan []byte]struct{}),
	}
	s.registerRoutes()
	return s
}

// Start starts the HTTP server with an embedded frontend SPA.
func (s *Server) Start(addr string, frontendFS fs.FS) error {
	if frontendFS != nil {
		s.mux.HandleFunc("/", spaHandler(frontendFS))
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	s.log.Info("Starting API server", "addr", addr)
	return httpServer.ListenAndServe()
}

// registerRoutes builds the mux and attaches API routes.
func (s *Server) registerRoutes() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionByID)
	mux.HandleFunc("/api/mcp/configs", s.handleMCPConfigs)
	mux.HandleFunc("/ws/stream", s.handleStream)
	s.mux = mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessions, err := storage.ListSessions(s.sessionsDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, sessions)
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if id == "" {
		http.Error(w, "session id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		session, err := storage.GetSession(s.sessionsDir, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, session)

	case http.MethodDelete:
		if err := storage.DeleteSession(s.sessionsDir, id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMCPConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configs, err := s.mcpStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, configs)
}

// handleStream upgrades the HTTP connection to a WebSocket and registers the
// client for event broadcasts. Blocks until the client disconnects.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if s.log != nil {
			s.log.Error("websocket upgrade failed", "error", err)
		}
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	events, err := s.eventbus.Subscribe(ctx, eventbus.TopicAgentStreamChunk)
	if err != nil {
		s.log.Error("failed to subscribe to events", "error", err)
		return
	}

	// Read loop (handle client messages / keep-alive)
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Write loop (forward events to client)
	// TODO: A centralized hub for multi-players, going for broadcast pattern?
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}
}

// clientCount returns the number of currently connected WebSocket clients.
// Intended for tests and observability.
func (s *Server) clientCount() int {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	return len(s.clients)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func spaHandler(frontendFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(frontendFS))
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if f, err := frontendFS.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
}
