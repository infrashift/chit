package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// The stream_status indicator (ui/activity): "chit-agent is responding…"
// on the lower border of the main pane, for the open channel.
const (
	// streamTTL ends an indicator whose sender stopped without "finished".
	streamTTL = 30 * time.Second
	// maxStreams is the most indicators at a time.
	maxStreams = 16
)

// streamKey names one stream of one sender. The sender comes first, so a
// post of the sender can clear each of its streams.
func streamKey(userID, streamID string) string {
	assert.That(userID != "", "streamKey: empty user ID")
	return userID + "/" + streamID
}

// onStreamStatus shows, refreshes or clears the indicator of a stream in
// the open channel.
func (m *Model) onStreamStatus(evt model.WebSocketEvent) []tea.Cmd {
	assert.That(m.appState == AppStateRunning, "onStreamStatus: not signed in")
	channelID := dataString(evt.Data, "channel_id")
	userID := dataString(evt.Data, "user_id")
	if m.activeChan == nil || channelID != m.activeChan.ID || userID == "" {
		return nil
	}
	key := streamKey(userID, dataString(evt.Data, "stream_id"))
	switch dataString(evt.Data, "status") {
	case model.StreamFinished:
		m.activity.Clear(key)
		return nil
	case model.StreamStarted, model.StreamProgress:
		name := m.usernameMap()[userID]
		if name == "" {
			name = "someone"
		}
		return []tea.Cmd{m.activity.Set(key, name+" is responding…")}
	}
	return nil
}

// endStreamsOf clears each indicator of userID: the reply has arrived.
func (m *Model) endStreamsOf(userID string) {
	if userID != "" {
		m.activity.ClearPrefix(userID + "/")
	}
	assert.That(m.activity.Len() <= maxStreams, "endStreamsOf: indicators over the cap")
}
