package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/honganh1206/tinker/store"
)

type Server struct {
	store      store.Store
	frontendFS fs.FS
	mux        *http.ServeMux
}

func New(s store.Store, frontendFS fs.FS) *Server {
	srv := &Server{store: s, frontendFS: frontendFS}
	srv.mux = srv.buildMux()
	return srv
}

func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	if s.frontendFS != nil {
		mux.HandleFunc("GET /", s.spaHandler())
	}
	return mux
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	result, err := s.store.Load(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) spaHandler() http.HandlerFunc {
	fileServer := http.FileServer(http.FS(s.frontendFS))
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		if f, err := s.frontendFS.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
