package apiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

// sessionCookieName is the name of the session cookie issued after login.
const sessionCookieName = "tinker_session"

// oauthStateCookieName is a short-lived CSRF token for the Google OAuth redirect.
const oauthStateCookieName = "tinker_oauth_state"

// sessionTTL is how long an issued session stays valid.
const sessionTTL = 7 * 24 * time.Hour

// oauthStateTTL is how long a started OAuth flow may be completed.
const oauthStateTTL = 10 * time.Minute

// userSession is a single authenticated user session.
type userSession struct {
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
	ExpiresAt time.Time `json:"-"`
}

// tokenVerifier validates a Google ID token credential and returns the
// payload claims. It exists so tests can inject a fake verifier.
type tokenVerifier interface {
	Verify(ctx context.Context, credential string) (*idtoken.Payload, error)
}

// googleVerifier is the production verifier backed by google.golang.org/api/idtoken.
type googleVerifier struct {
	clientID string
}

func (g *googleVerifier) Verify(ctx context.Context, credential string) (*idtoken.Payload, error) {
	return idtoken.Validate(ctx, credential, g.clientID)
}

type codeExchanger interface {
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
}

type oauthCodeExchanger struct {
	cfg *oauth2.Config
}

func (e *oauthCodeExchanger) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return e.cfg.Exchange(ctx, code)
}

// AuthConfig configures the Google login flow on the API server.
//
// When ClientID is empty, authentication is disabled and all routes are
// served anonymously. This is intended for local development.
type AuthConfig struct {
	// ClientID is the Google OAuth 2.0 client ID. Empty disables auth.
	ClientID string
	// ClientSecret is the Google OAuth 2.0 client secret.
	ClientSecret string
	// RedirectURL is the registered Google OAuth callback URL.
	RedirectURL string
	// Verifier overrides the default Google ID token verifier. Tests use this.
	Verifier tokenVerifier
}

// authenticator handles Google ID token verification and session storage.
type authenticator struct {
	verifier    tokenVerifier
	oauthConfig *oauth2.Config
	exchanger   codeExchanger

	mu       sync.RWMutex
	sessions map[string]userSession
}

// newAuthenticator constructs an authenticator from the given config.
// Returns nil when auth is disabled (cfg.ClientID == "").
func newAuthenticator(cfg AuthConfig) *authenticator {
	if cfg.ClientID == "" {
		return nil
	}
	verifier := cfg.Verifier
	if verifier == nil {
		verifier = &googleVerifier{clientID: cfg.ClientID}
	}
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
	return &authenticator{
		verifier:    verifier,
		oauthConfig: oauthConfig,
		exchanger:   &oauthCodeExchanger{cfg: oauthConfig},
		sessions:    make(map[string]userSession),
	}
}

func (a *authenticator) lookup(token string) (userSession, bool) {
	a.mu.RLock()
	sess, ok := a.sessions[token]
	a.mu.RUnlock()
	if !ok {
		return userSession{}, false
	}
	if time.Now().After(sess.ExpiresAt) {
		a.mu.Lock()
		delete(a.sessions, token)
		a.mu.Unlock()
		return userSession{}, false
	}
	return sess, true
}

func (a *authenticator) put(token string, sess userSession) {
	a.mu.Lock()
	a.sessions[token] = sess
	a.mu.Unlock()
}

func (a *authenticator) delete(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

// newSessionToken returns a cryptographically random opaque token.
func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sessionFromPayload(payload *idtoken.Payload) (userSession, error) {
	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return userSession{}, errors.New("credential missing email")
	}
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)
	return userSession{
		Email:   email,
		Name:    name,
		Picture: picture,
	}, nil
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/auth/google/callback",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

// handleGoogleStart starts the backend-owned Google OAuth authorization code flow.
func (s *Server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil {
		http.Error(w, "auth disabled", http.StatusNotFound)
		return
	}

	state, err := newSessionToken()
	if err != nil {
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}
	expires := time.Now().Add(oauthStateTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/auth/google/callback",
		Expires:  expires,
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	http.Redirect(w, r, s.auth.oauthConfig.AuthCodeURL(state), http.StatusFound)
}

// handleGoogleCallback completes the Google OAuth code flow and creates an app session.
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil {
		http.Error(w, "auth disabled", http.StatusNotFound)
		return
	}

	code := r.URL.Query().Get("code")
	returnedState := r.URL.Query().Get("state")
	stateCookie, err := r.Cookie(oauthStateCookieName)
	if code == "" || returnedState == "" || err != nil || stateCookie.Value == "" || stateCookie.Value != returnedState {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	clearOAuthStateCookie(w, r)

	oauthToken, err := s.auth.exchanger.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "failed to exchange oauth code", http.StatusUnauthorized)
		return
	}
	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		http.Error(w, "missing id token", http.StatusUnauthorized)
		return
	}
	payload, err := s.auth.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "invalid id token", http.StatusUnauthorized)
		return
	}
	sess, err := sessionFromPayload(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	token, err := newSessionToken()
	if err != nil {
		http.Error(w, "failed to issue session", http.StatusInternalServerError)
		return
	}
	sess.ExpiresAt = time.Now().Add(sessionTTL)
	s.auth.put(token, sess)
	setSessionCookie(w, r, token, sess.ExpiresAt)
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleLogout clears the session cookie and removes the server-side entry.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if c, err := r.Cookie(sessionCookieName); err == nil {
		s.auth.delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleMe returns the currently signed-in user, or 401 if not signed in.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil {
		writeJSON(w, map[string]any{"authDisabled": true})
		return
	}
	sess, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, sess)
}

// currentUser extracts the authenticated session from the request, if any.
func (s *Server) currentUser(r *http.Request) (userSession, bool) {
	if s.auth == nil {
		return userSession{}, false
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return userSession{}, false
	}
	return s.auth.lookup(c.Value)
}

// requireAuth wraps a handler so it only runs when the request carries a
// valid session cookie. When auth is disabled the handler is returned as-is.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	if s.auth == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.currentUser(r); !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
