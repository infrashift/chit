package login

import (
	"errors"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
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
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "login.New: styles not built by styles.New")
	id := textinput.New()
	id.Placeholder = "Email or username"
	id.CharLimit = 256
	id.Focus()

	pw := textinput.New()
	pw.Placeholder = "Password"
	pw.CharLimit = 256
	pw.EchoMode = textinput.EchoPassword
	pw.EchoCharacter = '*'
	assert.That(id.Focused(), "login.New: identifier not focused")
	assert.That(!pw.Focused(), "login.New: password focused")
	assert.That(pw.EchoMode == textinput.EchoPassword, "login.New: password echoed in clear")

	return Model{
		identifier:   id,
		password:     pw,
		kratosClient: kratosClient,
		styles:       s,
	}
}

// SetSize sets the available dimensions.
func (m *Model) SetSize(w, h int) {
	assert.That(w >= 0, "login.SetSize: negative width")
	assert.That(h >= 0, "login.SetSize: negative height")
	m.width = w
	m.height = h
}

// SetStyles updates the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "login.SetStyles: styles not built by styles.New")
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
	assert.That(m.identifier.Focused(), "login.Reset: identifier not focused")
	assert.That(!m.password.Focused(), "login.Reset: password still focused")
	assert.That(m.identifier.Value() == "", "login.Reset: identifier not cleared")
	assert.That(m.password.Value() == "", "login.Reset: password not cleared")
}

// Update handles messages for the login screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	assert.That(m.focusField >= 0 && m.focusField < numFields, "login.Update: focus field out of range")
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
		assert.That(msg.Err != nil, "login.Update: LoginErrorMsg without an error")
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
	assert.That(m.focusField >= 0 && m.focusField < numFields, "login.updateFocus: focus field out of range")
	m.identifier.Blur()
	m.password.Blur()
	switch m.focusField {
	case fieldIdentifier:
		m.identifier.Focus()
	case fieldPassword:
		m.password.Focus()
	}
	assert.That(m.identifier.Focused() != m.password.Focused(), "login.updateFocus: not exactly one field focused")
}

// View renders the login screen.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	title := m.styles.Username.Render("Chit Login")

	idLabel := m.styles.Timestamp.Render("Email / Username:")
	pwLabel := m.styles.Timestamp.Render("Password:")

	assert.That(m.state >= StateIdle && m.state <= StateError, "login.View: unknown state")
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
	assert.That(boxWidth <= 50, "login.View: box wider than 50")
	assert.That(boxWidth <= m.width, "login.View: box wider than the screen")

	box := m.styles.CmdPalette.Width(boxWidth).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}
