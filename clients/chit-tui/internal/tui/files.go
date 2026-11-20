package tui

import (
	"context"
	"maps"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// The records of the files that posts show (GET /files/{id}/info). The
// server allows a person 10 requests a second in bursts of 50, so the
// client asks for at most maxFileBatch records at a time, one batch after
// the other, and waits out a 429.
const (
	maxFileBatch = 40
	// maxFileQueue and maxFileCache bound the IDs that wait and the
	// records that the client keeps. A file past them shows its
	// placeholder.
	maxFileQueue = 400
	maxFileCache = 2000
	// minFileRetry is the shortest wait after a 429 with no Retry-After.
	minFileRetry = time.Second
)

// FileInfosLoadedMsg carries one batch of file records. Left holds the IDs
// that a 429 stopped, to ask for again after RetryAfter.
type FileInfosLoadedMsg struct {
	Files      []*model.File
	Left       []string
	RetryAfter time.Duration
	Err        error
}

func (m FileInfosLoadedMsg) requestError() error { return m.Err }

// fileRetryMsg asks for the next batch after a 429.
type fileRetryMsg struct{}

// FetchFileInfos asks for the record of each file in ids, in order. A file
// that the server does not show (404, 403) keeps its placeholder. A 429
// stops the batch, and a 401 ends it with the error.
func FetchFileInfos(ctx context.Context, client api.ChitClient, ids []string) tea.Cmd {
	assert.That(ctx != nil && client != nil, "FetchFileInfos: nil context or client")
	assert.That(len(ids) > 0 && len(ids) <= maxFileBatch, "FetchFileInfos: batch empty or too large")
	return func() tea.Msg {
		var msg FileInfosLoadedMsg
		for i, id := range ids {
			f, err := client.GetFileInfo(ctx, id)
			if wait, limited := api.RetryAfterOf(err); limited {
				msg.Left, msg.RetryAfter = ids[i:], max(wait, minFileRetry)
				return msg
			}
			if api.IsUnauthorized(err) {
				msg.Err = err
				return msg
			}
			if err == nil && f != nil {
				msg.Files = append(msg.Files, f)
			}
		}
		return msg
	}
}

// queuePostFiles queues the files of posts that have no record yet.
func (m *Model) queuePostFiles(posts []*model.Post) {
	assert.That(m.files != nil && m.filesRequested != nil, "queuePostFiles: nil file caches")
	for _, p := range posts {
		for _, id := range p.FileIDs {
			if m.files[id] != nil || m.filesRequested[id] || len(m.fileQueue) >= maxFileQueue {
				continue
			}
			m.filesRequested[id] = true
			m.fileQueue = append(m.fileQueue, id)
		}
	}
	assert.That(len(m.fileQueue) <= maxFileQueue, "queuePostFiles: queue over its cap")
}

// fetchMissingFiles starts the next batch, unless one is in flight.
func (m *Model) fetchMissingFiles() tea.Cmd {
	if m.fileFetching || len(m.fileQueue) == 0 {
		return nil
	}
	n := min(len(m.fileQueue), maxFileBatch)
	batch := append([]string(nil), m.fileQueue[:n]...)
	m.fileQueue = m.fileQueue[n:]
	m.fileFetching = true
	assert.That(len(batch) == n, "fetchMissingFiles: batch not copied whole")
	return FetchFileInfos(m.reqCtx(), m.client, batch)
}

// handleFileInfosLoaded keeps the records, gives them to the panes, and
// starts the next batch: at once, or after the wait of a 429.
func (m Model) handleFileInfosLoaded(msg FileInfosLoadedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.files != nil, "handleFileInfosLoaded: nil file cache")
	m.fileFetching = false
	for _, f := range msg.Files {
		if len(m.files) < maxFileCache {
			m.files[f.ID] = f
		}
	}
	if len(msg.Files) > 0 {
		m.pushFiles()
	}
	if len(msg.Left) > 0 {
		m.fileQueue = append(append([]string(nil), msg.Left...), m.fileQueue...)
		m.fileQueue = m.fileQueue[:min(len(m.fileQueue), maxFileQueue)]
		return m, tea.Tick(msg.RetryAfter, func(time.Time) tea.Msg { return fileRetryMsg{} })
	}
	return m, m.fetchMissingFiles()
}

// pushFiles gives a copy of the records to both panes. A copy, so that a
// pane can tell its old records from the new ones.
func (m *Model) pushFiles() {
	files := maps.Clone(m.files)
	m.viewport.SetFiles(files)
	m.thread.SetFiles(maps.Clone(files))
	assert.That(len(files) == len(m.files), "pushFiles: records lost in the copy")
}
