package main

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	"golang.org/x/oauth2"

	"github.com/gorilla/sessions"
	uuid "github.com/satori/go.uuid"
)

// OAuth implements user authorization via OAuth2.
type OAuth struct {
	*Logger
	oauth2.Config
	// Finally route to this url after success or error to authenticate.
	Finally string
	// Begin route to redirect non-logged in requests.
	Begin string
	// Session is the name of the session to be used.
	Session string
	// Sessions holds the session store implementation. This object is
	// responsible for saving and loading session objects.
	Sessions sessions.Store
	// Holds the state objects which are used to verify the authenticity of
	// the callback request.
	states *sync.Map
}

// Login via OAuth2.
func (a *OAuth) Login() http.HandlerFunc {
	a.states = &sync.Map{}
	return func(w http.ResponseWriter, r *http.Request) {
		state := uuid.NewV4().String()
		u := a.Config.AuthCodeURL(state)
		a.states.Store(state, true)
		http.Redirect(w, r, u, http.StatusTemporaryRedirect)
	}
}

// Callback to grab the OAuth code and state and open a session.
func (a *OAuth) Callback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer a.finally(w, r)
		state := r.FormValue("state")
		if _, ok := a.states.Load(state); !ok {
			a.Logf("invalid oauth state: %q\n", state)
			return
		}
		defer a.states.Delete(state)
		code := r.FormValue("code")
		token, err := a.Config.Exchange(oauth2.NoContext, code)
		if err != nil {
			a.Logf("exchanging code for token: %v\n", err)
			return
		}
		s, err := a.Sessions.New(r, a.Session)
		if err != nil {
			a.Logf("creating new session: %v", err)
			return
		}
		s.Values["id_token"] = token.Extra("id_token")
		if err := s.Save(r, w); err != nil {
			a.Logf("saving session: %v", err)
			return
		}

		// type response struct {
		// 	// Access string      `json:"access_token,omitempty"`
		// 	// Type   string      `json:"token_type,omitempty"`
		// 	// Expiry time.Time   `json:"expiry,omitempty"`
		// 	ID interface{} `json:"id_token,omitempty"`
		// }
		// resp := response{
		// 	ID: token.Extra("id_token"),
		// 	// Access: token.AccessToken,
		// 	// Type:   token.Type(),
		// 	// Expiry: token.Expiry,
		// }
		// if err := json.NewEncoder(w).Encode(resp); err != nil {
		// 	a.Logf("writing token to response writer: %v", err)
		// }
	}
}

// Secure middleware ensures user is authorized to access this route.
func (a *OAuth) Secure(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := a.Sessions.Get(r, a.Session)
		if err != nil {
			a.Logf("getting session for secured route: %v", err)
			a.finally(w, r)
			return
		}
		if token, ok := s.Values["id_token"].(string); !ok || token == "" {
			a.forbidden(w, r)
			return
		}
		// if sessioned, ok := h.(SessionHandler); ok {
		// 	sessioned.SetSession(s)
		// }
		h.ServeHTTP(w, r)
	}
}

// finally redirects to Finally route.
func (a *OAuth) finally(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, a.Finally, http.StatusTemporaryRedirect)
}

func (a *OAuth) forbidden(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, a.Finally, http.StatusForbidden)
}

// OAuthConfig holds the config data for an OAuth provider.
type OAuthConfig struct {
	ClientID string `json:"client_id"`
	Secret   string `json:"client_secret"`
}

// Logger logs things to a Writer.
type Logger struct {
	io.Writer
	Prefix string
	mu     sync.Mutex
}

// Logf writes formatted messages to the configured Writer unless nil.
func (l *Logger) Logf(f string, v ...interface{}) (int, error) {
	if l.Writer == nil {
		return 0, nil
	}
	if l.Prefix != "" {
		f = fmt.Sprintf("%s: %s", l.Prefix, f)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return fmt.Fprintf(l.Writer, f, v...)
}

// Prefixed returns a new logger that logs to the same Writer but with a
// different prefix.
func (l *Logger) Prefixed(prefix string) *Logger {
	return &Logger{
		Writer: l.Writer,
		Prefix: prefix,
	}
}
