package tui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/chips"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/filepick"
)

// Attachments (ISSUE-023, ADR-0018): /attach uploads an image to the open
// channel, a tray above the input holds the files of the next post, and the
// post sends their IDs.

const (
	subjectAltText = "alt-text:"
	// uploadKey prefixes the activity line of an upload.
	uploadKey = "upload:"
)

// imageTypes maps each file extension that the server accepts to the
// content type of the upload.
func imageTypes() map[string]string {
	out := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif"}
	assert.That(len(out) == 5, "imageTypes: an extension is missing")
	return out
}

// imageExts returns the extensions of imageTypes, for the file picker.
func imageExts() []string {
	exts := []string{".png", ".jpg", ".jpeg", ".webp", ".gif"}
	assert.That(len(exts) == len(imageTypes()), "imageExts: out of step with imageTypes")
	return exts
}

// FileUploadedMsg carries the record of an upload, or why it failed.
type FileUploadedMsg struct {
	Path string
	File *model.File
	Err  error
}

func (m FileUploadedMsg) requestError() error { return m.Err }

// UploadFileCmd sends the file at path to channelID.
func UploadFileCmd(ctx context.Context, client api.ChitClient, channelID, path, contentType, altText string) tea.Cmd {
	assert.That(ctx != nil && client != nil, "UploadFileCmd: nil context or client")
	assert.That(channelID != "" && path != "" && contentType != "", "UploadFileCmd: empty channel, path or type")
	return func() tea.Msg {
		f, err := os.Open(path)
		if err != nil {
			return FileUploadedMsg{Path: path, Err: err}
		}
		defer errlog.Close(f, "uploaded file")
		rec, err := client.UploadFile(ctx, channelID, filepath.Base(path), contentType, altText, f)
		return FileUploadedMsg{Path: path, File: rec, Err: err}
	}
}

// attachCommand runs /attach: with no path it opens the file picker, and
// with a path it asks for the alt text of that file.
func (m Model) attachCommand(args string) (tea.Model, tea.Cmd) {
	assert.That(m.attachTray.Len() <= model.MaxPostFiles, "attachCommand: tray over its cap")
	assert.That(m.appState == AppStateRunning, "attachCommand: not signed in")
	if m.activeChan == nil {
		return m, m.setError(errors.New("open a channel first"))
	}
	if m.attachTray.Full() {
		return m, m.setError(fmt.Errorf("a post holds at most %d files", model.MaxPostFiles))
	}
	path := strings.TrimSpace(args)
	if path == "" {
		start, err := os.UserHomeDir()
		if err != nil || start == "" {
			start = "."
		}
		return m, m.filePick.Open(start)
	}
	return m.askAltText(expandHome(path))
}

// expandHome replaces a leading "~/" with the home directory.
func expandHome(path string) string {
	assert.That(path != "", "expandHome: empty path")
	rest, ok := strings.CutPrefix(path, "~/")
	if !ok {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	return filepath.Join(home, rest)
}

// checkAttachable returns the content type of the image at path, or why the
// server would refuse it.
func checkAttachable(path string) (string, error) {
	assert.That(len(imageTypes()) == len(imageExts()), "checkAttachable: type list out of step")
	assert.That(path != "", "checkAttachable: empty path")
	contentType, ok := imageTypes()[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return "", fmt.Errorf("%s: attach a PNG, JPEG, WebP or GIF image", filepath.Base(path))
	}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("%s: no such file", filepath.Base(path))
	case err != nil:
		return "", err
	case info.IsDir():
		return "", fmt.Errorf("%s is a directory", path)
	case info.Size() > model.MaxFileBytes:
		return "", fmt.Errorf("%s is too large (10 MB at most)", filepath.Base(path))
	}
	return contentType, nil
}

// askAltText opens the prompt for the alt text of the file at path.
func (m Model) askAltText(path string) (tea.Model, tea.Cmd) {
	assert.That(path != "", "askAltText: empty path")
	if _, err := checkAttachable(path); err != nil {
		return m, m.setError(err)
	}
	cmd := m.textPrompt.Open(subjectAltText+path, "Alt text for "+filepath.Base(path), "",
		"say what the image shows · enter attaches · esc cancels")
	assert.That(m.textPrompt.Visible(), "askAltText: prompt not open")
	return m, cmd
}

// uploadWithAltText starts the upload of path with the alt text.
func (m Model) uploadWithAltText(path, altText string) (tea.Model, tea.Cmd) {
	assert.That(m.client != nil && m.scope != nil, "uploadWithAltText: no client or request scope")
	assert.That(path != "", "uploadWithAltText: empty path")
	contentType, err := checkAttachable(path)
	if err != nil {
		m.textPrompt.SetError(err.Error())
		return m, nil
	}
	if len([]rune(altText)) > api.MaxAltText {
		m.textPrompt.SetError(fmt.Sprintf("alt text holds at most %d characters", api.MaxAltText))
		return m, nil
	}
	m.textPrompt.Close()
	if m.activeChan == nil {
		return m, m.setError(errors.New("open a channel first"))
	}
	tick := m.activity.Set(uploadKey+path, "uploading "+filepath.Base(path)+"…")
	return m, tea.Batch(tick, UploadFileCmd(m.reqCtx(), m.client, m.activeChan.ID, path, contentType, altText))
}

// handleFileUploaded puts an uploaded file in the tray of its channel.
func (m Model) handleFileUploaded(msg FileUploadedMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.Path != "", "handleFileUploaded: no path")
	m.activity.Clear(uploadKey + msg.Path)
	if msg.Err != nil {
		return m, m.setError(fmt.Errorf("%s: %s", filepath.Base(msg.Path), api.UserMessage(msg.Err)))
	}
	assert.That(msg.File != nil && msg.File.ID != "", "handleFileUploaded: no record and no error")
	if msg.File.ChannelID != m.activeChannelID() {
		return m, nil // the user moved on; the server sweeps the upload
	}
	// The record is known now, so the post that takes the file shows it
	// with no fetch. The panes get it at once.
	if len(m.files) < maxFileCache {
		m.files[msg.File.ID] = msg.File
		m.pushFiles()
	}
	m.attachTray.Add(chips.Chip{ID: msg.File.ID, Label: "[file] " + msg.File.Name})
	m.syncTray()
	return m, nil
}

// handleFilePicked asks for the alt text of the picked file.
func (m Model) handleFilePicked(msg filepick.PickedMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.Path != "", "handleFilePicked: empty path")
	assert.That(msg.Size <= model.MaxFileBytes, "handleFilePicked: the picker passed a file over the cap")
	return m.askAltText(msg.Path)
}

// clearTray empties the tray: after a post took the files, after /detach,
// and on a channel switch.
func (m *Model) clearTray() {
	assert.That(m.attachTray.Len() <= model.MaxPostFiles, "clearTray: tray over its cap")
	m.attachTray.Clear()
	m.syncTray()
	assert.That(m.attachTray.Len() == 0, "clearTray: files left")
}

// syncTray lets the input send a post of files alone while the tray holds
// files.
func (m *Model) syncTray() {
	assert.That(m.appState == AppStateRunning || m.attachTray.Len() == 0, "syncTray: files in the tray before sign-in")
	m.input.SetSendEmpty(m.attachTray.Len() > 0)
	assert.That(m.attachTray.Len() <= model.MaxPostFiles, "syncTray: tray over its cap")
}

// popTray removes the last file of the tray: Backspace in an empty input.
func (m *Model) popTray(msg tea.KeyMsg) bool {
	assert.That(m.attachTray.Len() <= model.MaxPostFiles, "popTray: tray over its cap")
	if m.focus != FocusInput || msg.Type != tea.KeyBackspace || m.input.Value() != "" || m.attachTray.Len() == 0 {
		return false
	}
	_, ok := m.attachTray.Pop()
	m.syncTray()
	assert.That(ok, "popTray: no file to remove")
	return true
}

// placeTray draws the files of the tray on the upper border of the input.
func (m Model) placeTray(layout string) string {
	assert.That(layout != "", "placeTray: empty layout")
	if m.attachTray.Len() == 0 || m.width < 20 {
		return layout
	}
	line := m.attachTray.View() + m.styles.Timestamp.Render("  · backspace removes · /detach")
	line = ansi.Truncate(line, m.width-8, "…")
	y := m.height - inputHeight - 1
	assert.That(y < m.height, "placeTray: row below the screen")
	if y < 0 {
		return layout
	}
	return placeOverlay(3, y, " "+line+" ", layout)
}
