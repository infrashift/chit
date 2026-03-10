package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-User-Id", testKratosID) // matches TrustedProxyHeader default
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthExtract_MissingHeader(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	handler := AuthExtract(a)(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
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
		w.Write([]byte("ok"))
	})

	handler := StructuredLogger(next)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
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

	r := httptest.NewRequest(http.MethodGet, "/", nil)
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
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.RemoteAddr = "10.0.0.1:9999"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w1.Code)
	}

	// Second request should be rate limited
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.RemoteAddr = "10.0.0.1:9999"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", w2.Code)
	}
}
