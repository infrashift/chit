package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemPing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/system/ping", nil)
	w := httptest.NewRecorder()

	systemPing(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "OK" {
		t.Fatalf("expected status=OK, got %q", result["status"])
	}
}

func TestSystemClientConfig(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := systemClientConfig(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/system/config/client", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["version"]; !ok {
		t.Fatal("expected 'version' in response")
	}
}
