package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chitapi "github.com/infrashift/chit/api"

	"github.com/go-chi/chi/v5"
)

func TestDocsRoute_RawSpec(t *testing.T) {
	r := chi.NewRouter()
	MountDocs(r, chitapi.OpenAPISpec)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.yaml", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/yaml" {
		t.Fatalf("expected Content-Type application/yaml, got %s", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, chitapi.OpenAPISpec) {
		t.Fatal("response body does not match embedded spec")
	}
}

func TestDocsRoute_SwaggerUI(t *testing.T) {
	// MountDocs configures swagger with basePath=/api/v1/docs/ so we must
	// mount it under the same prefix the production router uses.
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		MountDocs(r, chitapi.OpenAPISpec)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/docs/", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html") {
		t.Fatal("expected Swagger UI HTML response")
	}
}
