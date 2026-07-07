package login

import (
	"errors"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit-tui/internal/auth"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
)

// State tracks the login screen state.
type State int

const (
	StateIdle State = iota
	StateLoading
	StateError
)

const (
	fieldIdentifier = 0
	fieldPassword   = 1
	numFields       = 2
)

// Model is the login screen component.
type Model struct {
	identifier   textinput.Model
	password     textinput.Model
	focusField   int
	state        State
	errorMsg     string
	kratosClient *auth.KratosClient
	styles       styles.Styles
	width        int
	height       int
}

// New creates a new login model.
func New(s styles.Styles, kratosClient *auth.KratosClient) Model {
	id := textinput.New()
	id.Placeholder = "Email or username"
	id.CharLimit = 256
	id.Focus()

	pw := textinput.New()
	pw.Placeholder = "Password"
	pw.CharLimit = 256
	pw.EchoMode = textinput.EchoPassword
	pw.EchoCharacter = '*'

	return Model{
		identifier:   id,
		password:     pw,
		kratosClient: kratosClient,
		styles:       s,
	}
}

// SetSize sets the available dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetStyles updates the styles.
func (m *Model) SetStyles(s styles.Styles) {
	m.styles = s
}

// Reset clears the form for re-login.
func (m *Model) Reset() {
	m.identifier.SetValue("")
	m.password.SetValue("")
	m.focusField = fieldIdentifier
	m.identifier.Focus()
	m.password.Blur()
	m.state = StateIdle
	m.errorMsg = ""
}

// Update handles messages for the login screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state == StateLoading {
			return m, nil
		}

		switch msg.Type {
		case tea.KeyTab, tea.KeyDown:
			m.focusField = (m.focusField + 1) % numFields
			m.updateFocus()
			return m, nil
		case tea.KeyShiftTab, tea.KeyUp:
			m.focusField = (m.focusField - 1 + numFields) % numFields
			m.updateFocus()
			return m, nil
		case tea.KeyEnter:
			id := m.identifier.Value()
			pw := m.password.Value()
			if id == "" || pw == "" {
				m.state = StateError
				m.errorMsg = "Both fields are required"
				return m, nil
			}
			m.state = StateLoading
			m.errorMsg = ""
			return m, InitLoginAndSubmit(m.kratosClient, id, pw)
		}

	case LoginErrorMsg:
		m.state = StateError
		var kratosErr *auth.KratosError
		if errors.As(msg.Err, &kratosErr) {
			m.errorMsg = kratosErr.Message
		} else {
			m.errorMsg = msg.Err.Error()
		}
		// Clear password on error for security
		m.password.SetValue("")
		return m, nil
	}

	var cmd tea.Cmd
	switch m.focusField {
	case fieldIdentifier:
		m.identifier, cmd = m.identifier.Update(msg)
	case fieldPassword:
		m.password, cmd = m.password.Update(msg)
	}
	return m, cmd
}

func (m *Model) updateFocus() {
	m.identifier.Blur()
	m.password.Blur()
	switch m.focusField {
	case fieldIdentifier:
		m.identifier.Focus()
	case fieldPassword:
		m.password.Focus()
	}
}

// View renders the login screen.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	title := m.styles.Username.Render("Chit Login")

	idLabel := m.styles.Timestamp.Render("Email / Username:")
	pwLabel := m.styles.Timestamp.Render("Password:")

	var statusLine string
	switch m.state {
	case StateLoading:
		statusLine = m.styles.Timestamp.Render("Signing in...")
	case StateError:
		statusLine = m.styles.ErrorText.Render(m.errorMsg)
	default:
		statusLine = ""
	}

	hints := m.styles.Timestamp.Render("Tab: switch fields  Enter: sign in  Ctrl+C: quit")

	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		"",
		idLabel,
		m.identifier.View(),
		"",
		pwLabel,
		m.password.View(),
		"",
		statusLine,
		"",
		hints,
	)

	boxWidth := 50
	if m.width < boxWidth+4 {
		boxWidth = m.width - 4
	}

	box := m.styles.CmdPalette.Width(boxWidth).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}
