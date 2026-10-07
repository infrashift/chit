package tui

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/login"
)

func (m Model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	// The listeners are one chain each, re-armed by every message they
	// deliver. Whatever arrives while signed out is stale, but dropping it
	// without re-arming would leave the next session deaf.
	switch msg.(type) {
	case WebSocketEventMsg:
		return m, ListenWebSocket(m.wsClient)
	case WSStateMsg:
		return m, ListenWSState(m.wsClient)
	}

	// Ctrl+C quits from login screen too.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, m.keys.Quit) {
		return m, tea.Quit
	}

	// Handle login success from async command.
	if successMsg, ok := msg.(login.LoginSuccessMsg); ok {
		return m.handleLoginSuccess(successMsg)
	}

	var cmd tea.Cmd
	m.loginModel, cmd = m.loginModel.Update(msg)
	return m, cmd
}

func (m Model) handleLoginSuccess(msg login.LoginSuccessMsg) (tea.Model, tea.Cmd) {
	if m.tokenStore != nil {
		m.tokenStore.Set(msg.Token)
	}
	if m.wsClient != nil {
		m.wsClient.SetToken(msg.Token)
	}

	// Persist session to disk.
	serverURL := ""
	if m.cfg != nil {
		serverURL = m.cfg.ServerURL
	}
	// A failed save means this login will not survive a restart. It does not
	// stop the session working now, so it is a warning rather than a failure.
	var saveCmd tea.Cmd
	if err := m.sessionStore.Save(auth.StoredSession{
		ServerURL: serverURL,
		Token:     msg.Token,
		ExpiresAt: msg.ExpiresAt,
	}); err != nil {
		saveCmd = m.setError(fmt.Errorf("signed in, but the session could not be saved: %w", err))
	}

	m.appState = AppStateRunning
	return m, tea.Batch(m.initRunning(), saveCmd)
}

// handleAuthExpired keeps the session's state for the re-login, which is
// usually the same user picking up where they left off. Who signs in is only
// known once their user loads; resumeAfterReLogin decides then.
func (m Model) handleAuthExpired() (tea.Model, tea.Cmd) {
	m.appState = AppStateReLogin
	m.wsConnected = false
	m.loginModel.Reset()
	if m.me != nil {
		m.expiredUserID = m.me.ID
	}
	// A stored token that cannot be cleared would be retried on next start
	// and fail the same way, so this is worth knowing about.
	if err := m.sessionStore.Clear(); err != nil {
		slog.Warn("could not clear the stored session", "error", err)
	}

	// Close existing WS connection.
	if m.wsClient != nil {
		_ = m.wsClient.Close()
	}

	return m, nil
}

func (m Model) handleLogout() (tea.Model, tea.Cmd) {
	if m.tokenStore != nil {
		m.tokenStore.Set("")
	}
	if m.wsClient != nil {
		_ = m.wsClient.Close()
	}

	// Whoever signs in next starts from nothing. Keeping the session showed
	// them the previous user's channel and history.
	m = m.newSession()
	m.appState = AppStateLogin

	// Logging out and leaving the token on disk would sign the user straight
	// back in on next start, which is the opposite of what they asked for.
	var clearCmd tea.Cmd
	if err := m.sessionStore.Clear(); err != nil {
		clearCmd = m.setError(fmt.Errorf("signed out, but the stored session remains: %w", err))
	}
	return m, clearCmd
}

// resumeAfterReLogin finishes a re-login once the signed-in user is known.
// The same user keeps their session, but anything posted while it was
// expired never arrived over the socket, so the open channel is re-read.
// Anyone else starts from nothing.
func (m Model) resumeAfterReLogin(u *model.User) (tea.Model, tea.Cmd) {
	same := u.ID == m.expiredUserID
	m.expiredUserID = ""
	if !same {
		m = m.newSession()
		m.appState = AppStateRunning
		return m.Update(UserLoadedMsg{User: u})
	}

	updated, cmd := m.Update(UserLoadedMsg{User: u})
	m = updated.(Model)
	if m.activeChan != nil {
		cmd = tea.Batch(cmd, m.selectChannel(m.activeChan))
	}
	return m, cmd
}

// newSession returns a model with nothing from the signed-in session: no
// user, channels, history, caches or half-finished edits. It keeps only what
// outlives a session — clients, stores, theme, window size, the status line,
// and the WebSocket listeners, which keep reading the same channels.
func (m Model) newSession() Model {
	fresh := NewModel(m.cfg, m.client, m.wsClient, m.styles, m.tokenStore, m.kratosClient, m.sessionStore)
	fresh.appState = m.appState
	fresh.wsListening = m.wsListening
	fresh.err, fresh.errSeq = m.err, m.errSeq
	fresh.width, fresh.height = m.width, m.height
	fresh.loginModel.SetSize(m.width, m.height)
	fresh.resizeComponents()
	return fresh
}
