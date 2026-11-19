// Package memberhooktest runs a membership hook and its token endpoint on
// httptest servers, for tests of the code that calls the hook.
package memberhooktest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/errlog"
	"github.com/infrashift/chit/internal/memberhook"
)

// Test credentials of the hook's OAuth2 client.
const (
	ClientID     = "chit-hook-test"
	ClientSecret = "chit-hook-test-secret"
)

// Answer is what the hook sends back: a status and a raw body.
type Answer struct {
	Status int
	Body   string
}

// Allow returns the answer {"allow":true}.
func Allow() Answer {
	return Answer{Status: http.StatusOK, Body: `{"allow":true}`}
}

// Refuse returns the answer {"allow":false,"reason":...,"detail":...}.
func Refuse(reason, detail string) Answer {
	b, err := json.Marshal(map[string]any{"allow": false, "reason": reason, "detail": detail})
	assert.That(err == nil, "memberhooktest: a map of strings always marshals")
	return Answer{Status: http.StatusOK, Body: string(b)}
}

// Server is a membership hook and a client_credentials token endpoint.
type Server struct {
	Hook  *httptest.Server
	Token *httptest.Server

	mu     sync.Mutex
	policy func(*memberhook.Request) Answer
	calls  []memberhook.Request
	tokens int
	// valid is the access token the hook accepts; each token request
	// issues a new one.
	valid string
}

// New starts a hook that answers with policy, and stops it when t ends.
func New(t *testing.T, policy func(*memberhook.Request) Answer) *Server {
	t.Helper()
	s := &Server{policy: policy}
	s.Token = httptest.NewServer(http.HandlerFunc(s.serveToken))
	s.Hook = httptest.NewServer(http.HandlerFunc(s.serveHook))
	t.Cleanup(s.Hook.Close)
	t.Cleanup(s.Token.Close)
	return s
}

// RefuseUser returns a policy that refuses userID and allows everyone else.
func RefuseUser(userID, reason, detail string) func(*memberhook.Request) Answer {
	return func(r *memberhook.Request) Answer {
		if r.UserID == userID {
			return Refuse(reason, detail)
		}
		return Allow()
	}
}

// Client returns a memberhook.Hook that calls this server.
func (s *Server) Client(t *testing.T) *memberhook.Hook {
	t.Helper()
	h, err := memberhook.New(&memberhook.Config{
		URL:          s.Hook.URL + "/internal/chit/membership-check",
		Timeout:      5 * time.Second,
		TokenURL:     s.Token.URL + "/oauth2/token",
		ClientID:     ClientID,
		ClientSecret: ClientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// SetPolicy replaces the policy.
func (s *Server) SetPolicy(policy func(*memberhook.Request) Answer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = policy
}

// Calls returns the requests the hook has received with a valid token, in
// order.
func (s *Server) Calls() []memberhook.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]memberhook.Request(nil), s.calls...)
}

// CalledFor returns the user IDs of the calls, in order.
func (s *Server) CalledFor() []string {
	var ids []string
	for _, c := range s.Calls() {
		ids = append(ids, c.UserID)
	}
	return ids
}

// Tokens returns how many tokens the token endpoint has issued.
func (s *Server) Tokens() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokens
}

func (s *Server) serveToken(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		// A form that does not parse leaves no client_id: the request is
		// refused below as an invalid client.
		errlog.Debug(r.ParseForm(), "memberhooktest: parse token form")
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != ClientID || secret != ClientSecret || r.FormValue("grant_type") != "client_credentials" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := w.Write([]byte(`{"error":"invalid_client"}`)); err != nil {
			errlog.Debug(err, "memberhooktest: write token refusal")
		}
		return
	}
	s.mu.Lock()
	s.tokens++
	s.valid = fmt.Sprintf("token-%d", s.tokens)
	token := s.valid
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"access_token": token, "token_type": "bearer", "expires_in": 3600,
	}); err != nil {
		errlog.Debug(err, "memberhooktest: write token")
	}
}

func (s *Server) serveHook(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	valid := s.valid
	s.mu.Unlock()
	if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+valid || valid == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req memberhook.Request
	if r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&req) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.calls = append(s.calls, req)
	policy := s.policy
	s.mu.Unlock()
	answer := policy(&req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(answer.Status)
	if _, err := w.Write([]byte(answer.Body)); err != nil {
		errlog.Debug(err, "memberhooktest: write hook answer")
	}
}
