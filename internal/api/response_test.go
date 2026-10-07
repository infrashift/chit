package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"hello": "world"}
	WriteJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type=application/json, got %q", ct)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["hello"] != "world" {
		t.Fatalf("expected hello=world, got %v", result)
	}
}

func TestWriteJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusNoContent, nil)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", w.Body.String())
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	appErr := model.NewBadRequestError("TestHandler", "invalid input")
	WriteError(w, appErr)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type=application/json, got %q", ct)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["id"] != "TestHandler" {
		t.Fatalf("expected id=%q, got %v", "TestHandler", result["id"])
	}
	if result["message"] != "invalid input" {
		t.Fatalf("expected message=%q, got %v", "invalid input", result["message"])
	}
}

func TestWriteAppError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"an AppError keeps its status", model.NewForbiddenError("x", "no"), http.StatusForbidden},
		{"a wrapped AppError keeps its status", fmt.Errorf("ctx: %w", model.NewNotFoundError("x", "id")), http.StatusNotFound},
		{"anything else is a 500", errors.New(`ERROR: relation "users" ... (SQLSTATE 42P01)`), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteAppError(w, "op", tc.err)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
		})
	}
}

// A 500's detail is the raw cause: pgx and SQL text the client cannot act on
// and should not see. It is logged, not sent.
func TestWriteError_ServerErrorsCarryNoDetail(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, model.NewInternalError("op", errors.New(`duplicate key value violates unique constraint "users_email_key"`)))
	if strings.Contains(w.Body.String(), "users_email_key") {
		t.Fatalf("a 500 leaked its cause: %s", w.Body.String())
	}
}

func TestDecodeBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"valid", `{"a":1}`, 0},
		{"malformed", `{a`, http.StatusBadRequest},
		{"over the size cap", `{"a":"` + strings.Repeat("x", maxBodyBytes) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			var v map[string]any
			ok := decodeBody(w, r, &v, "op")
			if ok != (tc.want == 0) || (tc.want != 0 && w.Code != tc.want) {
				t.Fatalf("ok=%v status=%d, want status %d", ok, w.Code, tc.want)
			}
		})
	}
}

func TestPagination(t *testing.T) {
	cases := []struct {
		query              string
		wantPage, wantSize int
	}{
		{"", 0, 60},
		{"?page=2&per_page=10", 2, 10},
		{"?page=-1&per_page=0", 0, 60},
		{"?per_page=-5", 0, 1},
		{"?per_page=100000", 0, maxPerPage},
		// page*per_page overflowed into a negative OFFSET, a 500 from Postgres.
		{"?page=9223372036854775807&per_page=2", maxOffset / 2, 2},
		{"?page=abc", 0, 60},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+tc.query, http.NoBody)
			page, size := parsePagination(r, 60)
			if page != tc.wantPage || size != tc.wantSize {
				t.Fatalf("got page=%d per_page=%d, want %d and %d", page, size, tc.wantPage, tc.wantSize)
			}
		})
	}
}
