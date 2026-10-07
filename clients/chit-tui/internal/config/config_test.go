package config_test

import (
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/config"
)

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "http://localhost:8065")
	t.Setenv("CHIT_SESSION_TOKEN", "test-token-123")
	t.Setenv("CHIT_WS_SCHEME", "")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerURL != "http://localhost:8065" {
		t.Errorf("ServerURL = %q", cfg.ServerURL)
	}
	if cfg.SessionToken != "test-token-123" {
		t.Errorf("SessionToken = %q", cfg.SessionToken)
	}
	if cfg.WSScheme != config.DefaultWSScheme {
		t.Errorf("WSScheme = %q, want %q", cfg.WSScheme, config.DefaultWSScheme)
	}
}

func TestLoad_MissingServerURL(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "")
	t.Setenv("CHIT_SESSION_TOKEN", "token")

	_, _, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing server URL")
	}
}

func TestLoad_MissingToken(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "http://localhost:8065")
	t.Setenv("CHIT_SESSION_TOKEN", "")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatalf("missing token should not be an error: %v", err)
	}
	if cfg.HasToken() {
		t.Error("HasToken() should return false when token is empty")
	}
}

func TestConfig_WSURL(t *testing.T) {
	cfg := &config.Config{
		ServerURL:    "http://localhost:8065",
		SessionToken: "tok",
		WSScheme:     "ws",
	}
	got := cfg.WSURL()
	want := "ws://localhost:8065/api/v1/websocket"
	if got != want {
		t.Errorf("WSURL() = %q, want %q", got, want)
	}
}

func TestConfig_WSURL_WSS(t *testing.T) {
	cfg := &config.Config{
		ServerURL:    "https://chat.example.com",
		SessionToken: "tok",
		WSScheme:     "wss",
	}
	got := cfg.WSURL()
	want := "wss://chat.example.com/api/v1/websocket"
	if got != want {
		t.Errorf("WSURL() = %q, want %q", got, want)
	}
}

func TestConfig_DefaultWSScheme(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "http://localhost:8065")
	t.Setenv("CHIT_SESSION_TOKEN", "tok")
	t.Setenv("CHIT_WS_SCHEME", "")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WSScheme != "ws" {
		t.Errorf("default WSScheme = %q, want ws", cfg.WSScheme)
	}
}

func TestConfig_DefaultAuthHeader(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "http://localhost:8065")
	t.Setenv("CHIT_SESSION_TOKEN", "tok")
	t.Setenv("CHIT_AUTH_HEADER", "")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthHeader != config.DefaultAuthHeader {
		t.Errorf("default AuthHeader = %q, want %q", cfg.AuthHeader, config.DefaultAuthHeader)
	}
}

func TestConfig_CustomAuthHeader(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "http://localhost:8065")
	t.Setenv("CHIT_SESSION_TOKEN", "tok")
	t.Setenv("CHIT_AUTH_HEADER", "X-User-Id")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthHeader != "X-User-Id" {
		t.Errorf("AuthHeader = %q, want %q", cfg.AuthHeader, "X-User-Id")
	}
}

func TestConfig_HasToken(t *testing.T) {
	cfg := &config.Config{ServerURL: "http://localhost:8065", SessionToken: "tok"}
	if !cfg.HasToken() {
		t.Error("HasToken() should return true when token is set")
	}
	cfg.SessionToken = ""
	if cfg.HasToken() {
		t.Error("HasToken() should return false when token is empty")
	}
}

func TestConfig_KratosBaseURL(t *testing.T) {
	cfg := &config.Config{ServerURL: "http://localhost:4455"}
	got := cfg.KratosBaseURL()
	want := "http://localhost:4455/kratos"
	if got != want {
		t.Errorf("KratosBaseURL() = %q, want %q", got, want)
	}
}

// A secure server defaulted to an insecure WebSocket: the dial failed, or the
// session token crossed the network in the clear.
func TestLoad_HTTPSServerDefaultsToSecureWebSocket(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "https://chat.example.com")
	t.Setenv("CHIT_WS_SCHEME", "")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.WSURL(), "wss://chat.example.com/api/v1/websocket"; got != want {
		t.Errorf("WSURL = %q, want %q", got, want)
	}
}

func TestLoad_ExplicitWSSchemeStillWins(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "https://chat.example.com")
	t.Setenv("CHIT_WS_SCHEME", "ws")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WSScheme != "ws" {
		t.Errorf("WSScheme = %q, want the explicit ws", cfg.WSScheme)
	}
}

// "http://host/" produced "http://host//api/v1".
func TestLoad_TrailingSlashIsDropped(t *testing.T) {
	t.Setenv("CHIT_SERVER_URL", "http://localhost:4455/")
	t.Setenv("CHIT_WS_SCHEME", "")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerURL != "http://localhost:4455" {
		t.Errorf("ServerURL = %q", cfg.ServerURL)
	}
	if got, want := cfg.WSURL(), "ws://localhost:4455/api/v1/websocket"; got != want {
		t.Errorf("WSURL = %q, want %q", got, want)
	}
}
