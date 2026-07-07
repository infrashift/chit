package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configDirName   = "chit-tui"
	sessionFile     = "session.json"
	dirPermissions  = 0700
	filePermissions = 0600
)

// StoredSession holds the data persisted to disk.
type StoredSession struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// SessionStore manages session persistence at a configurable file path.
// Use NewSessionStore to create one with an optional override path.
type SessionStore struct {
	path string // empty means use default
}

// NewSessionStore creates a SessionStore. If overridePath is non-empty, it is
// used as the session file path instead of the default
// (~/.config/chit-tui/session.json).
func NewSessionStore(overridePath string) *SessionStore {
	return &SessionStore{path: overridePath}
}

func (ss *SessionStore) resolve() (string, error) {
	if ss.path != "" {
		return ss.path, nil
	}
	dir, err := sessionDir()
	if err != nil {
		return "", err
	}
	return sessionPath(dir), nil
}

// Save writes a session to disk.
func (ss *SessionStore) Save(s StoredSession) error {
	p, err := ss.resolve()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), dirPermissions); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, filePermissions)
}

// Load reads a stored session from disk.
func (ss *SessionStore) Load() (*StoredSession, error) {
	p, err := ss.resolve()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s StoredSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Clear removes the stored session file.
func (ss *SessionStore) Clear() error {
	p, err := ss.resolve()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Package-level functions for backward compatibility with tests.

// SaveSession writes a session using the default path.
func SaveSession(s StoredSession) error { return NewSessionStore("").Save(s) }

// LoadSession reads a session using the default path.
func LoadSession() (*StoredSession, error) { return NewSessionStore("").Load() }

// ClearSession removes the session at the default path.
func ClearSession() error { return NewSessionStore("").Clear() }

func sessionDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, configDirName), nil
}

func sessionPath(dir string) string {
	return filepath.Join(dir, sessionFile)
}
