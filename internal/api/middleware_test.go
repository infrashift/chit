package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/model"
)

const (
	agentUserID       = "019421a0-0000-7000-8000-000000000004"
	testOAuthClientID = "8f2b1c4e-hydra-generated-client"
)

func TestAuthExtract_Success(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	// The next handler checks that the user is in context
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if user == nil {
			t.Error("expected user in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if user.ID != testUserID {
			t.Errorf("expected user ID=%q, got %q", testUserID, user.ID)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthExtract(a)(next)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-User-Id", testKratosID) // matches TrustedProxyHeader default
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// An agent authenticated by Hydra client-credentials arrives as a client ID,
// which chitd resolves against users.oauth_client_id.
func TestAuthExtract_OAuthClientResolvesToUser(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()

	ms.Users.Seed(&model.User{
		ID:            agentUserID,
		KratosID:      "kratos-agent-001",
		Username:      "claude-architect",
		DisplayName:   "Claude Architect",
		Email:         "architect@example.com",
		Roles:         "system_user",
		ActorType:     model.ActorTypeAgent,
		OAuthClientID: testOAuthClientID,
		CreateAt:      1000,
		UpdateAt:      1000,
	})

	var got *model.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = ContextGetUser(r)
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-Client-Id", testOAuthClientID)
	w := httptest.NewRecorder()
	AuthExtract(a)(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got == nil || got.ID != agentUserID {
		t.Fatalf("expected the agent user in context, got %+v", got)
	}
}

// An unrecognised client must be rejected, never provisioned into a user the
// way an unknown Kratos ID is.
func TestAuthExtract_UnknownOAuthClientRejected(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called for an unknown client")
	})

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-Client-Id", "client-that-was-never-bound")
	w := httptest.NewRecorder()
	AuthExtract(a)(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// The client header wins, so a forged X-User-Id cannot ride along with a
// legitimately introspected token.
func TestAuthExtract_ClientHeaderBeatsForgedUserHeader(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()

	ms.Users.Seed(&model.User{
		ID:            agentUserID,
		KratosID:      "kratos-agent-001",
		Username:      "claude-architect",
		DisplayName:   "Claude Architect",
		Email:         "architect@example.com",
		Roles:         "system_user",
		ActorType:     model.ActorTypeAgent,
		OAuthClientID: testOAuthClientID,
		CreateAt:      1000,
		UpdateAt:      1000,
	})

	var got *model.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = ContextGetUser(r)
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-Client-Id", testOAuthClientID)
	r.Header.Set("X-User-Id", testKratosID) // would be a different, human user
	w := httptest.NewRecorder()
	AuthExtract(a)(next).ServeHTTP(w, r)

	if got == nil || got.ID != agentUserID {
		t.Fatalf("client header must win; got %+v", got)
	}
}

func TestAuthExtract_MissingHeader(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	handler := AuthExtract(a)(next)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	// No X-User-Id header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestStructuredLogger(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := StructuredLogger(next)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestResponseWriter_HijackDelegatesToUnderlying(t *testing.T) {
	// httptest.NewRecorder does NOT implement http.Hijacker, so wrapping
	// it should surface that error.
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	_, _, err := rw.Hijack()
	if err == nil {
		t.Fatal("expected error when underlying writer is not a Hijacker")
	}
}

func TestRateLimit_Allows(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(100, 100)(next)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRateLimit_Blocks(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Very low rate: 1 per second, burst 1
	handler := RateLimit(1, 1)(next)

	// First request should pass
	r1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r1.RemoteAddr = "10.0.0.1:9999"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w1.Code)
	}

	// Second request should be rate limited
	r2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r2.RemoteAddr = "10.0.0.1:9999"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", w2.Code)
	}
}

func TestAuthExtract_ProxySecretRequired(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	a.Config.TrustedProxySecret = "s3cret"

	mw := AuthExtract(a)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Correct user header but missing proxy secret → 401.
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-User-Id", testKratosID)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing secret: expected 401, got %d", w.Code)
	}

	// Wrong secret → 401.
	r = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-User-Id", testKratosID)
	r.Header.Set("X-Proxy-Secret", "wrong")
	w = httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: expected 401, got %d", w.Code)
	}

	// Correct secret → passes through.
	r = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-User-Id", testKratosID)
	r.Header.Set("X-Proxy-Secret", "s3cret")
	w = httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("correct secret: expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// Idle buckets are evicted once they would have refilled anyway; a busy
// caller's bucket is never reset (the old janitor wiped every bucket every
// ten minutes, refilling the burst of whoever was active).
func TestUserLimiters_EvictsOnlyIdleBuckets(t *testing.T) {
	now := time.Unix(0, 0)
	clock := func() time.Time { return now }
	u := newUserLimiters(1, 2, clock)

	for i := range 2 {
		if !u.allow("busy") {
			t.Fatalf("request %d of a burst of 2 refused", i+1)
		}
	}
	if !u.allow("idle") {
		t.Fatal("first request refused")
	}
	if u.allow("busy") {
		t.Fatal("a third request inside a second was allowed with a burst of 2")
	}

	// Long enough for "idle" to be evicted; "busy" keeps calling.
	for range 3 {
		now = now.Add(u.idleAfter / 2)
		u.allow("busy")
	}
	u.mu.Lock()
	_, idleKept := u.entries["idle"]
	_, busyKept := u.entries["busy"]
	u.mu.Unlock()
	if idleKept || !busyKept {
		t.Fatalf("idle kept=%v busy kept=%v, want only the busy bucket", idleKept, busyKept)
	}
}
