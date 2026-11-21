package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// Save the files of a post to download_dir ("s" in the post details).
// A file never overwrites another: a taken name gets a number.

// maxNameTries bounds the names that saveFile tries for one file.
const maxNameTries = 100

// FilesSavedMsg carries the paths of the saved files.
type FilesSavedMsg struct {
	Dir   string
	Paths []string
	Err   error
}

func (m FilesSavedMsg) requestError() error { return m.Err }

// SaveFilesCmd downloads each file in ids to dir. names gives the name of
// each file whose record has loaded.
func SaveFilesCmd(ctx context.Context, client api.ChitClient, dir string, ids []string, names map[string]string) tea.Cmd {
	assert.That(ctx != nil && client != nil, "SaveFilesCmd: nil context or client")
	assert.That(dir != "" && len(ids) > 0 && len(ids) <= model.MaxPostFiles, "SaveFilesCmd: no directory, or 0 or too many files")
	return func() tea.Msg {
		msg := FilesSavedMsg{Dir: dir}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			msg.Err = err
			return msg
		}
		for _, id := range ids {
			path, err := saveFile(ctx, client, dir, id, names[id])
			if err != nil {
				msg.Err = err
				return msg
			}
			msg.Paths = append(msg.Paths, path)
		}
		return msg
	}
}

// saveFile downloads one file to a new file in dir and returns its path.
// A failed download removes the part that it wrote.
func saveFile(ctx context.Context, client api.ChitClient, dir, id, name string) (string, error) {
	assert.That(id != "", "saveFile: empty file ID")
	f, path, err := createFree(dir, safeName(name, id))
	if err != nil {
		return "", err
	}
	_, err = client.DownloadFile(ctx, id, f, model.MaxFileBytes)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		errlog.Debug(os.Remove(path), "remove a part of a failed download")
		return "", err
	}
	assert.That(strings.HasPrefix(path, dir), "saveFile: path outside the directory")
	return path, nil
}

// safeName returns the base of name, with no path, or the start of the ID
// when name is empty.
func safeName(name, id string) string {
	assert.That(id != "", "safeName: empty file ID")
	base := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if base == "." || base == "/" || base == ".." || base == "" {
		base = "file-" + id[:min(8, len(id))]
	}
	assert.That(!strings.ContainsAny(base, `/\`), "safeName: a path in the name")
	return base
}

// createFree creates a new file for name in dir, never over another:
// "chart.png", then "chart (1).png", up to maxNameTries.
func createFree(dir, name string) (*os.File, string, error) {
	assert.That(dir != "" && name != "", "createFree: empty directory or name")
	assert.That(!strings.ContainsAny(name, `/\`), "createFree: a path in the name")
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := range maxNameTries {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		path := filepath.Join(dir, candidate)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			return f, path, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("%s: %d names are taken in %s", name, maxNameTries, dir)
}

// saveInfoFiles saves the files of the post in the details.
func (m Model) saveInfoFiles() (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "saveInfoFiles: not signed in")
	p := m.infoPost
	if p == nil || len(p.FileIDs) == 0 {
		return m, nil
	}
	names := make(map[string]string, len(p.FileIDs))
	for _, id := range p.FileIDs {
		if f := m.files[id]; f != nil {
			names[id] = f.Name
		}
	}
	assert.That(m.downloadDir != "", "saveInfoFiles: no download directory")
	return m, SaveFilesCmd(m.reqCtx(), m.client, m.downloadDir, p.FileIDs, names)
}

// handleFilesSaved tells where the files went.
func (m Model) handleFilesSaved(msg FilesSavedMsg) (tea.Model, tea.Cmd) {
	assert.That(len(msg.Paths) <= model.MaxPostFiles, "handleFilesSaved: more files than a post holds")
	assert.That(msg.Dir != "", "handleFilesSaved: no directory")
	if msg.Err != nil {
		return m, m.setError(fmt.Errorf("save: %s", api.UserMessage(msg.Err)))
	}
	m.infoBox.Close()
	noun := "files"
	if len(msg.Paths) == 1 {
		noun = "file"
	}
	return m, m.setError(fmt.Errorf("saved %d %s to %s", len(msg.Paths), noun, shortDir(msg.Dir)))
}

// shortDir writes the home directory at the start of dir as "~".
func shortDir(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return dir
	}
	if rest, ok := strings.CutPrefix(dir, home); ok && (rest == "" || strings.HasPrefix(rest, string(filepath.Separator))) {
		return "~" + rest
	}
	assert.That(dir != "", "shortDir: empty directory")
	return dir
}
