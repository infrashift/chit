package auth_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/infrashift/chit-tui/internal/auth"
)

func TestSaveAndLoadSession(t *testing.T) {
	// Use a temporary config dir to avoid writing to the real one.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	s := auth.StoredSession{
		ServerURL: "http://localhost:4455",
		Token:     "test-token-123",
		ExpiresAt: "2026-12-31T23:59:59Z",
	}

	if err := auth.SaveSession(s); err != nil {
		t.Fatal(err)
	}

	loaded, err := auth.LoadSession()
	if err != nil {
		t.Fatal(err)
	}

	if loaded.ServerURL != s.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, s.ServerURL)
	}
	if loaded.Token != s.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, s.Token)
	}
	if loaded.ExpiresAt != s.ExpiresAt {
		t.Errorf("ExpiresAt = %q, want %q", loaded.ExpiresAt, s.ExpiresAt)
	}
}

func TestSaveSession_FilePermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	s := auth.StoredSession{
		ServerURL: "http://localhost:4455",
		Token:     "secret",
		ExpiresAt: "2026-12-31T23:59:59Z",
	}

	if err := auth.SaveSession(s); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmp, "chit-tui", "session.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestLoadSession_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	_, err := auth.LoadSession()
	if err == nil {
		t.Fatal("expected error when no session file exists")
	}
}

func TestLoadSession_CorruptFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, "chit-tui")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session.json"), []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := auth.LoadSession()
	if err == nil {
		t.Fatal("expected error for corrupt session file")
	}
}

func TestClearSession(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	s := auth.StoredSession{
		ServerURL: "http://localhost:4455",
		Token:     "to-be-cleared",
		ExpiresAt: "2026-12-31T23:59:59Z",
	}

	if err := auth.SaveSession(s); err != nil {
		t.Fatal(err)
	}

	if err := auth.ClearSession(); err != nil {
		t.Fatal(err)
	}

	_, err := auth.LoadSession()
	if err == nil {
		t.Fatal("expected error after clearing session")
	}
}

func TestClearSession_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Should not error when no file exists.
	if err := auth.ClearSession(); err != nil {
		t.Fatalf("ClearSession() should not error when no file exists: %v", err)
	}
}

func TestSaveSession_Overwrite(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	s1 := auth.StoredSession{ServerURL: "http://a", Token: "old", ExpiresAt: "2026-01-01T00:00:00Z"}
	if err := auth.SaveSession(s1); err != nil {
		t.Fatal(err)
	}

	s2 := auth.StoredSession{ServerURL: "http://b", Token: "new", ExpiresAt: "2026-06-01T00:00:00Z"}
	if err := auth.SaveSession(s2); err != nil {
		t.Fatal(err)
	}

	loaded, err := auth.LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != "new" {
		t.Errorf("Token = %q, want %q", loaded.Token, "new")
	}
	if loaded.ServerURL != "http://b" {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, "http://b")
	}
}

func TestSaveSession_NoConfigDir(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	s := auth.StoredSession{ServerURL: "http://x", Token: "tok", ExpiresAt: "2026-01-01T00:00:00Z"}
	err := auth.SaveSession(s)
	if err == nil {
		t.Fatal("expected error when config dir is unavailable")
	}
}

func TestLoadSession_NoConfigDir(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	_, err := auth.LoadSession()
	if err == nil {
		t.Fatal("expected error when config dir is unavailable")
	}
}

func TestClearSession_NoConfigDir(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	err := auth.ClearSession()
	if err == nil {
		t.Fatal("expected error when config dir is unavailable")
	}
}

func TestSaveAndClear_Cycle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	s := auth.StoredSession{ServerURL: "http://x", Token: "tok", ExpiresAt: "2026-01-01T00:00:00Z"}
	if err := auth.SaveSession(s); err != nil {
		t.Fatal(err)
	}
	if err := auth.ClearSession(); err != nil {
		t.Fatal(err)
	}
	// Save again after clear.
	if err := auth.SaveSession(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := auth.LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != "tok" {
		t.Errorf("Token = %q, want %q", loaded.Token, "tok")
	}
}

func TestSessionStore_OverridePath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "custom-session.json")
	ss := auth.NewSessionStore(path)

	s := auth.StoredSession{ServerURL: "http://custom", Token: "custom-tok", ExpiresAt: "2026-12-31T00:00:00Z"}
	if err := ss.Save(s); err != nil {
		t.Fatal(err)
	}

	loaded, err := ss.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != "custom-tok" {
		t.Errorf("Token = %q, want %q", loaded.Token, "custom-tok")
	}

	if err := ss.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.Load(); err == nil {
		t.Fatal("expected error after clear")
	}
}

func TestSessionStore_TwoStoresIsolated(t *testing.T) {
	tmp := t.TempDir()
	ss1 := auth.NewSessionStore(filepath.Join(tmp, "alice.json"))
	ss2 := auth.NewSessionStore(filepath.Join(tmp, "bob.json"))

	s1 := auth.StoredSession{Token: "alice-tok"}
	s2 := auth.StoredSession{Token: "bob-tok"}

	if err := ss1.Save(s1); err != nil {
		t.Fatal(err)
	}
	if err := ss2.Save(s2); err != nil {
		t.Fatal(err)
	}

	loaded1, _ := ss1.Load()
	loaded2, _ := ss2.Load()

	if loaded1.Token != "alice-tok" {
		t.Errorf("store1 Token = %q, want alice-tok", loaded1.Token)
	}
	if loaded2.Token != "bob-tok" {
		t.Errorf("store2 Token = %q, want bob-tok", loaded2.Token)
	}
}
