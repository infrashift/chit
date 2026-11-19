package auth

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
)

const (
	// legacyDirName is where the session lived before it moved beside the
	// config and themes in the chit config directory.
	legacyDirName   = "chit-tui"
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
// used as the session file path instead of the default,
// session.json in the chit config directory (~/.config/chit on Linux).
func NewSessionStore(overridePath string) *SessionStore {
	return &SessionStore{path: overridePath}
}

func (ss *SessionStore) resolve() (string, error) {
	assert.That(ss != nil, "SessionStore.resolve: nil store")
	if ss.path != "" {
		return ss.path, nil
	}
	return filepath.Join(config.Dir(), sessionFile), nil
}

// legacyPath is where the default session file used to be, or "" when the
// store has an explicit path and so never used it.
func (ss *SessionStore) legacyPath() string {
	assert.That(ss != nil, "SessionStore.legacyPath: nil store")
	if ss.path != "" {
		return ""
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, legacyDirName, sessionFile)
}

// Save writes a session to disk.
func (ss *SessionStore) Save(s StoredSession) error {
	assert.That(ss != nil, "SessionStore.Save: nil store")
	p, err := ss.resolve()
	if err != nil {
		return err
	}
	assert.That(p != "", "SessionStore.Save: empty session path")
	if err := os.MkdirAll(filepath.Dir(p), dirPermissions); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	assert.That(len(data) > 0, "SessionStore.Save: empty encoding")
	return writeFileAtomic(p, data)
}

// writeFileAtomic replaces path with data, readable by the owner alone.
// WriteFile applies its mode only when it creates a file, so an existing
// session file kept whatever looser mode it had, token and all; and a write
// interrupted halfway left a truncated file that no longer parsed.
func writeFileAtomic(path string, data []byte) error {
	assert.That(path != "", "writeFileAtomic: empty path")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	assert.That(filepath.Dir(name) == filepath.Dir(path), "writeFileAtomic: temp file must be beside the target for the rename to be atomic")
	if err := tmp.Chmod(filePermissions); err != nil {
		errlog.Debug(tmp.Close(), "close temp session file")
		errlog.Debug(os.Remove(name), "remove temp session file")
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		errlog.Debug(tmp.Close(), "close temp session file")
		errlog.Debug(os.Remove(name), "remove temp session file")
		return err
	}
	if err := tmp.Close(); err != nil {
		errlog.Debug(os.Remove(name), "remove temp session file")
		return err
	}
	if err := os.Rename(name, path); err != nil {
		errlog.Debug(os.Remove(name), "remove temp session file")
		return err
	}
	return nil
}

// Load reads a stored session from disk.
func (ss *SessionStore) Load() (*StoredSession, error) {
	assert.That(ss != nil, "SessionStore.Load: nil store")
	p, err := ss.resolve()
	if err != nil {
		return nil, err
	}
	assert.That(p != "", "SessionStore.Load: empty session path")
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		data, err = ss.moveLegacy(p)
	}
	if err != nil {
		return nil, err
	}
	var s StoredSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// moveLegacy moves a session left in the old directory to p and returns its
// contents, so upgrading does not sign anyone out. With nothing to move it
// reports p as not existing, as reading it did.
func (ss *SessionStore) moveLegacy(p string) ([]byte, error) {
	assert.That(p != "", "SessionStore.moveLegacy: empty session path")
	legacy := ss.legacyPath()
	if legacy == "" {
		return nil, &os.PathError{Op: "open", Path: p, Err: os.ErrNotExist}
	}
	// Only the default location has a legacy predecessor.
	assert.That(ss.path == "", "SessionStore.moveLegacy: a legacy path for an explicit store")
	assert.That(legacy != p, "SessionStore.moveLegacy: legacy and current paths are the same")
	data, err := os.ReadFile(legacy)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), dirPermissions); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(p, data); err != nil {
		return nil, err
	}
	// Moved; the old copy would only be found again after a sign-out.
	errlog.Debug(os.Remove(legacy), "remove legacy session file")
	// Removes the directory only if it is now empty; otherwise the error is
	// expected.
	errlog.Debug(os.Remove(filepath.Dir(legacy)), "remove legacy session directory")
	return data, nil
}

// Clear removes the stored session file. A session still in the old
// directory goes too: left there, the next start would move it over and
// sign the user straight back in.
func (ss *SessionStore) Clear() error {
	assert.That(ss != nil, "SessionStore.Clear: nil store")
	p, err := ss.resolve()
	if err != nil {
		return err
	}
	assert.That(p != "", "SessionStore.Clear: empty session path")
	if legacy := ss.legacyPath(); legacy != "" {
		assert.That(legacy != p, "SessionStore.Clear: legacy and current paths are the same")
		if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
