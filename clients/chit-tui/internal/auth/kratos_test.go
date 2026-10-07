package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
)

func kratosServer(t *testing.T, mux *http.ServeMux) (*auth.KratosClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return auth.NewKratosClient(srv.URL), srv
}

func TestInitLoginFlow_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "flow-123",
			"expires_at": "2026-12-31T23:59:59Z",
		})
	})
	client, _ := kratosServer(t, mux)

	flow, err := client.InitLoginFlow(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if flow.ID != "flow-123" {
		t.Errorf("flow.ID = %q, want %q", flow.ID, "flow-123")
	}
	if flow.ExpiresAt.IsZero() {
		t.Error("flow.ExpiresAt should not be zero")
	}
}

func TestInitLoginFlow_NetworkError(t *testing.T) {
	client := auth.NewKratosClient("http://127.0.0.1:1") // nothing listening
	_, err := client.InitLoginFlow(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestSubmitLogin_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /self-service/login", func(w http.ResponseWriter, r *http.Request) {
		flowID := r.URL.Query().Get("flow")
		if flowID != "flow-123" {
			http.Error(w, "bad flow", 400)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", 400)
			return
		}
		if body["identifier"] != "alice@example.com" || body["password"] != "secret" {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"message": "invalid credentials"},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_token": "tok-abc",
			"session": map[string]any{
				"expires_at": "2026-12-31T23:59:59Z",
				"identity": map[string]any{
					"id": "id-alice",
					"traits": map[string]string{
						"email":        "alice@example.com",
						"username":     "alice",
						"display_name": "Alice Smith",
					},
				},
			},
		})
	})
	client, _ := kratosServer(t, mux)

	sess, err := client.SubmitLogin(context.Background(), "flow-123", "alice@example.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Token != "tok-abc" {
		t.Errorf("Token = %q, want %q", sess.Token, "tok-abc")
	}
	if sess.Identity.ID != "id-alice" {
		t.Errorf("Identity.ID = %q, want %q", sess.Identity.ID, "id-alice")
	}
	if sess.Identity.Traits.Username != "alice" {
		t.Errorf("Traits.Username = %q, want %q", sess.Identity.Traits.Username, "alice")
	}
}

func TestSubmitLogin_WrongPassword(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /self-service/login", func(w http.ResponseWriter, _ *http.Request) {
		// Kratos returns 400 with the flow object containing ui.messages on bad credentials.
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "flow-123",
			"ui": map[string]any{
				"nodes": []any{},
				"messages": []map[string]any{
					{
						"id":   4000006,
						"text": "The provided credentials are invalid, check for spelling mistakes in your password or username, email address, or phone number.",
						"type": "error",
					},
				},
			},
		})
	})
	client, _ := kratosServer(t, mux)

	_, err := client.SubmitLogin(context.Background(), "flow-123", "alice@example.com", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	kratosErr, ok := err.(*auth.KratosError)
	if !ok {
		t.Fatalf("expected *KratosError, got %T", err)
	}
	if kratosErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", kratosErr.StatusCode)
	}
	if !strings.Contains(kratosErr.Message, "provided credentials are invalid") {
		t.Errorf("expected user-friendly message, got: %q", kratosErr.Message)
	}
}

func TestSubmitLogin_ExpiredFlow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /self-service/login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(410)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "flow expired"},
		})
	})
	client, _ := kratosServer(t, mux)

	_, err := client.SubmitLogin(context.Background(), "expired-flow", "alice@example.com", "secret")
	if err == nil {
		t.Fatal("expected error for expired flow")
	}
	kratosErr, ok := err.(*auth.KratosError)
	if !ok {
		t.Fatalf("expected *KratosError, got %T", err)
	}
	if kratosErr.StatusCode != 410 {
		t.Errorf("StatusCode = %d, want 410", kratosErr.StatusCode)
	}
}

func TestCheckSession_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sessions/whoami", func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("X-Session-Token")
		if tok != "valid-tok" {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"message": "unauthorized"},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{
				"id": "id-alice",
				"traits": map[string]string{
					"email":    "alice@example.com",
					"username": "alice",
				},
			},
		})
	})
	client, _ := kratosServer(t, mux)

	identity, err := client.CheckSession(context.Background(), "valid-tok")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ID != "id-alice" {
		t.Errorf("Identity.ID = %q, want %q", identity.ID, "id-alice")
	}
}

func TestCheckSession_InvalidToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sessions/whoami", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "unauthorized"},
		})
	})
	client, _ := kratosServer(t, mux)

	_, err := client.CheckSession(context.Background(), "bad-tok")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain 401, got: %v", err)
	}
}

func TestKratosError_Error(t *testing.T) {
	err := &auth.KratosError{StatusCode: 401, Message: "unauthorized"}
	got := err.Error()
	want := "kratos: 401 unauthorized"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInitLoginFlow_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "internal error"},
		})
	})
	client, _ := kratosServer(t, mux)

	_, err := client.InitLoginFlow(context.Background())
	if err == nil {
		t.Fatal("expected error for server error")
	}
	kratosErr, ok := err.(*auth.KratosError)
	if !ok {
		t.Fatalf("expected *KratosError, got %T", err)
	}
	if kratosErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", kratosErr.StatusCode)
	}
}

func TestCheckSession_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sessions/whoami", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("not json at all"))
	})
	client, _ := kratosServer(t, mux)

	_, err := client.CheckSession(context.Background(), "some-tok")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSubmitLogin_NetworkError(t *testing.T) {
	client := auth.NewKratosClient("http://127.0.0.1:1")
	_, err := client.SubmitLogin(context.Background(), "flow-1", "alice", "pass")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestCheckSession_NetworkError(t *testing.T) {
	client := auth.NewKratosClient("http://127.0.0.1:1")
	_, err := client.CheckSession(context.Background(), "tok")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestParseError_EmptyMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{},
		})
	})
	client, _ := kratosServer(t, mux)

	_, err := client.InitLoginFlow(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseError_UIMessages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "flow-abc",
			"ui": map[string]any{
				"messages": []map[string]any{
					{"text": "Account locked", "type": "error"},
				},
			},
		})
	})
	client, _ := kratosServer(t, mux)

	_, err := client.InitLoginFlow(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Account locked") {
		t.Errorf("expected 'Account locked' in error, got: %v", err)
	}
}

func TestParseError_TopLevelMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(422)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "validation failed",
		})
	})
	client, _ := kratosServer(t, mux)

	_, err := client.InitLoginFlow(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected 'validation failed' in error, got: %v", err)
	}
}

// A rejected login reports why on the form's fields (ui.nodes), not in a
// top-level message. That case fell through to showing the raw flow JSON.
func TestSubmitLogin_ReportsFieldErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /self-service/login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "flow-1",
			"ui": map[string]any{
				"nodes": []any{
					map[string]any{"messages": []any{}},
					map[string]any{"messages": []any{map[string]any{"text": "The password must be at least 8 characters long."}}},
				},
			},
		})
	})
	client, _ := kratosServer(t, mux)

	_, err := client.SubmitLogin(context.Background(), "flow-1", "alice", "short")
	if err == nil || !strings.Contains(err.Error(), "at least 8 characters") {
		t.Fatalf("err = %v, want the field's message", err)
	}
	if strings.Contains(err.Error(), "{") {
		t.Errorf("err = %v, want no raw JSON", err)
	}
}

// Only Kratos saying no to a session means it is gone. An unreachable or
// failing Kratos says nothing about the session.
func TestIsSessionRejected(t *testing.T) {
	if !auth.IsSessionRejected(&auth.KratosError{StatusCode: http.StatusUnauthorized}) {
		t.Error("a 401 is a rejection")
	}
	if auth.IsSessionRejected(&auth.KratosError{StatusCode: http.StatusBadGateway}) {
		t.Error("a 502 is not")
	}
	_, err := auth.NewKratosClient("http://127.0.0.1:1").CheckSession(context.Background(), "tok")
	if auth.IsSessionRejected(err) {
		t.Error("an unreachable Kratos is not")
	}
}
