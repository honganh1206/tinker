package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

// fakeVerifier is a deterministic tokenVerifier for tests.
type fakeVerifier struct {
	accept  string
	payload *idtoken.Payload
	err     error
}

func (f *fakeVerifier) Verify(_ context.Context, credential string) (*idtoken.Payload, error) {
	if f.err != nil {
		return nil, f.err
	}
	if credential != f.accept {
		return nil, errors.New("bad credential")
	}
	return f.payload, nil
}

// setupAuthServer returns a server with auth enabled and a controllable verifier.
func setupAuthServer(t *testing.T, payload *idtoken.Payload) (*Server, *fakeVerifier) {
	t.Helper()
	verifier := &fakeVerifier{accept: "good-token", payload: payload}
	cfg := AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://example.com/auth/google/callback",
		Verifier:     verifier,
	}
	s := NewServer(nil, nil, t.TempDir(), t.TempDir(), cfg)
	return s, verifier
}

type fakeCodeExchanger struct {
	token *oauth2.Token
	err   error
}

func (f *fakeCodeExchanger) Exchange(_ context.Context, code string) (*oauth2.Token, error) {
	if f.err != nil {
		return nil, f.err
	}
	if code != "good-code" {
		return nil, errors.New("bad code")
	}
	return f.token, nil
}

func TestRequireAuth_Unauthorized(t *testing.T) {
	s, _ := setupAuthServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_Disabled_AllowsAnonymous(t *testing.T) {
	// AuthConfig{} disables auth.
	s := NewServer(nil, nil, t.TempDir(), t.TempDir(), AuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGoogleStart_RedirectsToGoogleAndSetsStateCookie(t *testing.T) {
	s, _ := setupAuthServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/start", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, "accounts.google.com")
	assert.Contains(t, location, "client_id=test-client")
	assert.Contains(t, location, "redirect_uri=http%3A%2F%2Fexample.com%2Fauth%2Fgoogle%2Fcallback")
	assert.Contains(t, location, "scope=")
	assert.Contains(t, location, "state=")

	var stateCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthStateCookieName {
			stateCookie = c
			break
		}
	}
	require.NotNil(t, stateCookie)
	assert.NotEmpty(t, stateCookie.Value)
	assert.True(t, stateCookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, stateCookie.SameSite)
}

func TestGoogleCallback_RejectsMissingStateCookie(t *testing.T) {
	s, _ := setupAuthServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc&state=state", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGoogleCallback_RejectsMismatchedState(t *testing.T) {
	s, _ := setupAuthServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc&state=returned", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "stored"})
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGoogleCallback_SuccessCreatesSessionAndRedirectsHome(t *testing.T) {
	payload := &idtoken.Payload{
		Claims: map[string]any{
			"email":   "user@example.com",
			"name":    "User Example",
			"picture": "https://pic",
		},
	}
	s, _ := setupAuthServer(t, payload)
	s.auth.exchanger = &fakeCodeExchanger{
		token: (&oauth2.Token{}).WithExtra(map[string]any{"id_token": "good-token"}),
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=good-code&state=state", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state"})
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))

	var sessCookie *http.Cookie
	var clearedState *http.Cookie
	for _, c := range w.Result().Cookies() {
		switch c.Name {
		case sessionCookieName:
			sessCookie = c
		case oauthStateCookieName:
			clearedState = c
		}
	}
	require.NotNil(t, sessCookie)
	assert.True(t, sessCookie.HttpOnly)
	require.NotNil(t, clearedState)
	assert.Equal(t, -1, clearedState.MaxAge)

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.AddCookie(sessCookie)
	meW := httptest.NewRecorder()
	s.mux.ServeHTTP(meW, meReq)

	require.Equal(t, http.StatusOK, meW.Code)
	var got userSession
	require.NoError(t, json.NewDecoder(meW.Body).Decode(&got))
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, "User Example", got.Name)
}

func TestGoogleCallback_RejectsMissingIDToken(t *testing.T) {
	s, _ := setupAuthServer(t, nil)
	s.auth.exchanger = &fakeCodeExchanger{token: &oauth2.Token{}}

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=good-code&state=state", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state"})
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogout_ClearsSession(t *testing.T) {
	payload := &idtoken.Payload{Claims: map[string]any{"email": "u@e.com"}}
	s, _ := setupAuthServer(t, payload)
	s.auth.exchanger = &fakeCodeExchanger{
		token: (&oauth2.Token{}).WithExtra(map[string]any{"id_token": "good-token"}),
	}

	loginReq := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=good-code&state=state", nil)
	loginReq.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state"})
	loginW := httptest.NewRecorder()
	s.mux.ServeHTTP(loginW, loginReq)
	require.Equal(t, http.StatusFound, loginW.Code)

	var sessCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == sessionCookieName {
			sessCookie = c
			break
		}
	}
	require.NotNil(t, sessCookie)

	// Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.AddCookie(sessCookie)
	logoutW := httptest.NewRecorder()
	s.mux.ServeHTTP(logoutW, logoutReq)
	assert.Equal(t, http.StatusNoContent, logoutW.Code)

	// Original cookie no longer works.
	req2 := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	req2.AddCookie(sessCookie)
	w2 := httptest.NewRecorder()
	s.mux.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestMe_Unauthorized(t *testing.T) {
	s, _ := setupAuthServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
