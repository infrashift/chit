package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chitapi "github.com/infrashift/chit/api"

	"github.com/go-chi/chi/v5"
)

func TestValidationMiddleware_Init(t *testing.T) {
	mw, err := NewValidationMiddleware(chitapi.OpenAPISpec)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if mw == nil {
		t.Fatal("expected non-nil middleware")
	}
}

func TestValidationMiddleware_InvalidSpec(t *testing.T) {
	_, err := NewValidationMiddleware([]byte("garbage"))
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
}

func TestValidationMiddleware_ValidRequest(t *testing.T) {
	mw, err := NewValidationMiddleware(chitapi.OpenAPISpec)
	if err != nil {
		t.Fatalf("failed to create middleware: %v", err)
	}

	r := chi.NewRouter()
	r.Use(mw)
	r.Get("/api/v1/system/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid request, got %d", w.Code)
	}
}

func TestValidationMiddleware_InvalidBody(t *testing.T) {
	mw, err := NewValidationMiddleware(chitapi.OpenAPISpec)
	if err != nil {
		t.Fatalf("failed to create middleware: %v", err)
	}

	r := chi.NewRouter()
	r.Use(mw)
	r.Post("/api/v1/teams", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// POST with wrong content-type (text/plain instead of application/json)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 400 or 415 for invalid body, got %d", w.Code)
	}
}
