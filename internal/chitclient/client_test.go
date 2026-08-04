package chitclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// fakeHydra is a minimal OAuth2 token endpoint implementing the
// client_credentials grant.
type fakeHydra struct {
	mu       sync.Mutex
	issued   int
	lastForm url.Values
	lastAuth string
	expires  int
}

func (h *fakeHydra) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		h.mu.Lock()
		h.issued++
		h.lastForm = r.PostForm
		h.lastAuth = r.Header.Get("Authorization")
		n := h.issued
		expires := h.expires
		h.mu.Unlock()

		if expires == 0 {
			expires = 3600
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("access-token-%d", n),
			"token_type":   "bearer",
			"expires_in":   expires,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (h *fakeHydra) tokensIssued() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.issued
}

func TestOAuthClientSendsBearerToken(t *testing.T) {
	hydra := &fakeHydra{}
	hydraSrv := hydra.server(t)

	var gotAuth, gotUserID string
	chitd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUserID = r.Header.Get("X-User-Id")
		_ = json.NewEncoder(w).Encode(&model.User{ID: "agent-1", Username: "claude-architect"})
	}))
	t.Cleanup(chitd.Close)

	c := NewOAuth(chitd.URL, OAuthConfig{
		TokenURL:     hydraSrv.URL,
		ClientID:     "generated-client-id",
		ClientSecret: "s3cret",
		Scopes:       []string{"chit:read", "chit:write"},
		Audience:     "chit",
	})

	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.ID != "agent-1" {
		t.Errorf("unexpected user: %+v", me)
	}
	// oauth2 normalises the token_type to the RFC 6750 spelling.
	if gotAuth != "Bearer access-token-1" {
		t.Errorf("Authorization = %q, want the issued bearer token", gotAuth)
	}
	// The trusted-proxy headers must not be sent in OAuth2 mode: identity
	// comes from the introspected token, not an asserted ID.
	if gotUserID != "" {
		t.Errorf("X-User-Id must not be sent in OAuth2 mode, got %q", gotUserID)
	}

	// The audience and scopes reach the token endpoint.
	hydra.mu.Lock()
	form := hydra.lastForm
	hydra.mu.Unlock()
	if got := form.Get("audience"); got != "chit" {
		t.Errorf("audience = %q, want chit", got)
	}
	if got := form.Get("scope"); got != "chit:read chit:write" {
		t.Errorf("scope = %q", got)
	}
}

// Hydra enforces the client's registered token_endpoint_auth_method and
// rejects the other outright, so the client must not pin one. Registering a
// client as client_secret_post while the client sent HTTP Basic produced
// "invalid_client" against a real Hydra.
func TestOAuthWorksWithPostOnlyTokenEndpoint(t *testing.T) {
	var sawBasicRejected bool
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if _, _, ok := r.BasicAuth(); ok {
			// Mirror Hydra: a post-only client refuses Basic auth.
			sawBasicRejected = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		if r.PostForm.Get("client_secret") == "" {
			http.Error(w, "no credentials", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "post-style-token", "token_type": "bearer", "expires_in": 3600,
		})
	}))
	t.Cleanup(tokenSrv.Close)

	var gotAuth string
	chitd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(&model.User{ID: "agent-1"})
	}))
	t.Cleanup(chitd.Close)

	c := NewOAuth(chitd.URL, OAuthConfig{
		TokenURL: tokenSrv.URL, ClientID: "cid", ClientSecret: "sec",
		Scopes: []string{"chit:read"}, Audience: "chit",
	})

	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("client must fall back to client_secret_post: %v", err)
	}
	if gotAuth != "Bearer post-style-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if !sawBasicRejected {
		t.Log("note: client used post style directly without probing Basic")
	}
}

// A cached token is reused across requests rather than re-fetched each time.
func TestOAuthTokenIsReused(t *testing.T) {
	hydra := &fakeHydra{}
	hydraSrv := hydra.server(t)

	chitd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&model.User{ID: "agent-1"})
	}))
	t.Cleanup(chitd.Close)

	c := NewOAuth(chitd.URL, OAuthConfig{
		TokenURL: hydraSrv.URL, ClientID: "cid", ClientSecret: "sec",
		Scopes: []string{"chit:read"}, Audience: "chit",
	})

	for range 3 {
		if _, err := c.Me(context.Background()); err != nil {
			t.Fatalf("Me: %v", err)
		}
	}
	if n := hydra.tokensIssued(); n != 1 {
		t.Errorf("expected the token to be cached across requests, got %d issuances", n)
	}
}

// A token endpoint that is down must fail the request, never fall through to
// sending it unauthenticated.
func TestOAuthTokenFailureFailsRequest(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "hydra is down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(dead.Close)

	called := false
	chitd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(chitd.Close)

	c := NewOAuth(chitd.URL, OAuthConfig{
		TokenURL: dead.URL, ClientID: "cid", ClientSecret: "sec",
		Scopes: []string{"chit:read"}, Audience: "chit",
	})

	if _, err := c.Me(context.Background()); err == nil {
		t.Fatal("expected an error when the token endpoint is unavailable")
	}
	if called {
		t.Error("request must not reach chitd without a token")
	}
}

// Trusted-proxy mode is unchanged: no Authorization header, identity asserted.
func TestTrustedProxyModeUnchanged(t *testing.T) {
	var gotAuth, gotUserID, gotSecret string
	chitd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUserID = r.Header.Get("X-User-Id")
		gotSecret = r.Header.Get("X-Proxy-Secret")
		_ = json.NewEncoder(w).Encode(&model.User{ID: "agent-1"})
	}))
	t.Cleanup(chitd.Close)

	c := New(chitd.URL, "kratos-agent-1", "shared-secret")
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("no Authorization header expected, got %q", gotAuth)
	}
	if gotUserID != "kratos-agent-1" || gotSecret != "shared-secret" {
		t.Errorf("proxy headers wrong: user=%q secret=%q", gotUserID, gotSecret)
	}
}
