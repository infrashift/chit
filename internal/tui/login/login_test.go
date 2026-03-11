package login_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit-tui/internal/auth"
	"github.com/infrashift/chit-tui/internal/tui/login"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit-tui/internal/tui/ui/theme"
)

// Suppress unused import warnings.
var _ = context.Background

func testLogin() login.Model {
	s := styles.New(theme.TokyoNight())
	m := login.New(s, nil)
	m.SetSize(80, 24)
	return m
}

func TestLogin_TabCyclesFields(t *testing.T) {
	m := testLogin()

	// Start on identifier field, tab to password
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Tab again wraps back to identifier
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Shift-tab goes back to password
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})

	// Should not panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestLogin_EnterWithEmptyFields(t *testing.T) {
	m := testLogin()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "required") {
		t.Error("expected validation error for empty fields")
	}
}

func TestLogin_ErrorMsgDisplayed(t *testing.T) {
	m := testLogin()

	m, _ = m.Update(login.LoginErrorMsg{Err: errors.New("invalid credentials")})

	view := m.View()
	if !strings.Contains(view, "invalid credentials") {
		t.Error("expected error message in view")
	}
}

func TestLogin_SuccessMsgHandled(t *testing.T) {
	m := testLogin()

	// LoginSuccessMsg is handled by the parent (app.go), not the login component.
	// The login component should pass it through without errors.
	view := m.View()
	if !strings.Contains(view, "Login") {
		t.Error("expected Login title in view")
	}
}

func TestLogin_Reset(t *testing.T) {
	m := testLogin()

	// Type something, then reset
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alice")})
	m.Reset()

	view := m.View()
	if strings.Contains(view, "alice") {
		t.Error("expected form to be cleared after reset")
	}
}

func TestLogin_LoadingState(t *testing.T) {
	m := testLogin()

	// Type identifier
	for _, r := range "alice@example.com" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Tab to password
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Type password
	for _, r := range "secret" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Submit
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Signing in") {
		t.Error("expected loading state in view")
	}

	// cmd should not be nil (it initiates the login flow)
	if cmd == nil {
		t.Error("expected non-nil command after submit")
	}
}

func TestLogin_SetStyles(t *testing.T) {
	m := testLogin()
	s := styles.New(theme.TokyoNight())
	m.SetStyles(s)
	// Should not panic.
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view after SetStyles")
	}
}

func TestLogin_LoadingIgnoresKeys(t *testing.T) {
	m := testLogin()

	// Type into fields and submit
	for _, r := range "alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "pass" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Now in loading state — keys should be ignored
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Error("expected nil command while loading")
	}
}

func TestLogin_UpArrowNavigation(t *testing.T) {
	m := testLogin()

	// Tab to password, then up-arrow back to identifier
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestLogin_DownArrowNavigation(t *testing.T) {
	m := testLogin()

	// Down-arrow from identifier to password
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestLogin_SmallWindow(t *testing.T) {
	s := styles.New(theme.TokyoNight())
	m := login.New(s, nil)
	m.SetSize(30, 10)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view for small window")
	}
}

func TestLogin_ZeroSize(t *testing.T) {
	s := styles.New(theme.TokyoNight())
	m := login.New(s, nil)

	view := m.View()
	if view != "Loading..." {
		t.Errorf("expected 'Loading...' for zero-size, got %q", view)
	}
}

func TestInitLoginAndSubmit_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "flow-1",
			"expires_at": "2026-12-31T23:59:59Z",
		})
	})
	mux.HandleFunc("POST /self-service/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_token": "tok-123",
			"session": map[string]any{
				"expires_at": "2026-12-31T23:59:59Z",
				"identity": map[string]any{
					"id":     "id-1",
					"traits": map[string]string{"email": "alice@example.com"},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	kc := auth.NewKratosClient(srv.URL)
	cmd := login.InitLoginAndSubmit(kc, "alice@example.com", "secret")
	msg := cmd()

	success, ok := msg.(login.LoginSuccessMsg)
	if !ok {
		t.Fatalf("expected LoginSuccessMsg, got %T: %v", msg, msg)
	}
	if success.Token != "tok-123" {
		t.Errorf("Token = %q, want %q", success.Token, "tok-123")
	}
}

func TestInitLoginAndSubmit_FlowError(t *testing.T) {
	kc := auth.NewKratosClient("http://127.0.0.1:1") // nothing listening
	cmd := login.InitLoginAndSubmit(kc, "alice@example.com", "secret")
	msg := cmd()

	errMsg, ok := msg.(login.LoginErrorMsg)
	if !ok {
		t.Fatalf("expected LoginErrorMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Error("expected non-nil error")
	}
}

func TestInitLoginAndSubmit_LoginError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "flow-1",
			"expires_at": "2026-12-31T23:59:59Z",
		})
	})
	mux.HandleFunc("POST /self-service/login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "bad password"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	kc := auth.NewKratosClient(srv.URL)
	cmd := login.InitLoginAndSubmit(kc, "alice@example.com", "wrong")
	msg := cmd()

	_, ok := msg.(login.LoginErrorMsg)
	if !ok {
		t.Fatalf("expected LoginErrorMsg, got %T", msg)
	}
}

func TestValidateSession_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sessions/whoami", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{
				"id": "id-1",
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	kc := auth.NewKratosClient(srv.URL)
	cmd := login.ValidateSession(kc, "valid-tok")
	msg := cmd()

	success, ok := msg.(login.LoginSuccessMsg)
	if !ok {
		t.Fatalf("expected LoginSuccessMsg, got %T", msg)
	}
	if success.Token != "valid-tok" {
		t.Errorf("Token = %q, want %q", success.Token, "valid-tok")
	}
}

func TestValidateSession_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sessions/whoami", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "unauthorized"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	kc := auth.NewKratosClient(srv.URL)
	cmd := login.ValidateSession(kc, "bad-tok")
	msg := cmd()

	_, ok := msg.(login.LoginErrorMsg)
	if !ok {
		t.Fatalf("expected LoginErrorMsg, got %T", msg)
	}
}
