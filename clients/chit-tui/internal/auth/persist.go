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
	return writeFileAtomic(p, data)
}

// writeFileAtomic replaces path with data, readable by the owner alone.
// WriteFile applies its mode only when it creates a file, so an existing
// session file kept whatever looser mode it had, token and all; and a write
// interrupted halfway left a truncated file that no longer parsed.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Chmod(filePermissions); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
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
