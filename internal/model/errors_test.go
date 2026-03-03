package model

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestNewAppError(t *testing.T) {
	e := NewAppError("TestWhere", "test message", "details here", 418)
	if e.ID != "TestWhere" {
		t.Fatalf("expected ID=%q, got %q", "TestWhere", e.ID)
	}
	if e.Where != "TestWhere" {
		t.Fatalf("expected Where=%q, got %q", "TestWhere", e.Where)
	}
	if e.Message != "test message" {
		t.Fatalf("expected Message=%q, got %q", "test message", e.Message)
	}
	if e.DetailedError != "details here" {
		t.Fatalf("expected DetailedError=%q, got %q", "details here", e.DetailedError)
	}
	if e.StatusCode != 418 {
		t.Fatalf("expected StatusCode=418, got %d", e.StatusCode)
	}
}

func TestAppError_Error(t *testing.T) {
	e := NewAppError("MyFunc", "something broke", "", 500)
	got := e.Error()
	if got != "MyFunc: something broke" {
		t.Fatalf("expected %q, got %q", "MyFunc: something broke", got)
	}
}

func TestAppError_ToJSON(t *testing.T) {
	e := NewAppError("TestJSON", "json message", "detail", 400)
	raw := e.ToJSON()

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("ToJSON produced invalid JSON: %v", err)
	}

	if parsed["id"] != "TestJSON" {
		t.Fatalf("expected id=%q, got %v", "TestJSON", parsed["id"])
	}
	if parsed["message"] != "json message" {
		t.Fatalf("expected message=%q, got %v", "json message", parsed["message"])
	}
	if int(parsed["status_code"].(float64)) != 400 {
		t.Fatalf("expected status_code=400, got %v", parsed["status_code"])
	}
	// Where is json:"-" so should not appear
	if _, ok := parsed["where"]; ok {
		t.Fatal("Where should not be serialized (json:\"-\")")
	}
}

func TestNewNotFoundError(t *testing.T) {
	e := NewNotFoundError("TestNF", "abc-123")
	if e.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", e.StatusCode)
	}
	if !strings.Contains(e.DetailedError, "abc-123") {
		t.Fatalf("expected DetailedError to contain the id, got %q", e.DetailedError)
	}
}

func TestNewBadRequestError(t *testing.T) {
	e := NewBadRequestError("TestBR", "bad input")
	if e.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", e.StatusCode)
	}
}

func TestNewUnauthorizedError(t *testing.T) {
	e := NewUnauthorizedError("TestUA", "no creds")
	if e.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", e.StatusCode)
	}
}

func TestNewForbiddenError(t *testing.T) {
	e := NewForbiddenError("TestFB", "denied")
	if e.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", e.StatusCode)
	}
}

func TestNewConflictError(t *testing.T) {
	e := NewConflictError("TestCF", "already exists")
	if e.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", e.StatusCode)
	}
}

func TestNewInternalError_NilErr(t *testing.T) {
	e := NewInternalError("TestIE", nil)
	if e.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", e.StatusCode)
	}
	if e.DetailedError != "" {
		t.Fatalf("expected empty DetailedError for nil err, got %q", e.DetailedError)
	}
}

func TestNewInternalError_WithErr(t *testing.T) {
	e := NewInternalError("TestIE", errors.New("db down"))
	if e.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", e.StatusCode)
	}
	if e.DetailedError != "db down" {
		t.Fatalf("expected DetailedError=%q, got %q", "db down", e.DetailedError)
	}
}
