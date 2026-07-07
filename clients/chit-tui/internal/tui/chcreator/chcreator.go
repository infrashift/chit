package chcreator

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)

const (
	fieldDisplayName = iota
	fieldName
	fieldType
	fieldPurpose
	fieldCount
)

// ChannelSubmittedMsg is sent when the form is submitted.
type ChannelSubmittedMsg struct {
	Channel *model.Channel
}

// Model is the channel creator overlay component.
type Model struct {
	displayName textinput.Model
	name        textinput.Model
	purpose     textinput.Model
	channelType string // "O" or "P"
	teamID      string
	activeField int
	autoSlug    bool
	visible     bool
	focused     bool
	validErr    string
	styles      styles.Styles
	width       int
	height      int
}

// New creates a new channel creator model.
func New(s styles.Styles) Model {
	dn := textinput.New()
	dn.Placeholder = "Display Name"
	dn.CharLimit = 64

	nm := textinput.New()
	nm.Placeholder = "channel-name"
	nm.CharLimit = 64

	pp := textinput.New()
	pp.Placeholder = "Purpose (optional)"
	pp.CharLimit = 250

	return Model{
		displayName: dn,
		name:        nm,
		purpose:     pp,
		channelType: model.ChannelOpen,
		autoSlug:    true,
		styles:      s,
	}
}

// Open shows the channel creator overlay with the given team ID.
func (m *Model) Open(teamID string) {
	m.visible = true
	m.focused = true
	m.teamID = teamID
	m.activeField = fieldDisplayName
	m.channelType = model.ChannelOpen
	m.autoSlug = true
	m.validErr = ""
	m.displayName.Reset()
	m.name.Reset()
	m.purpose.Reset()
	m.displayName.Focus()
	m.name.Blur()
	m.purpose.Blur()
}

// Close hides the channel creator overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.displayName.Blur()
	m.name.Blur()
	m.purpose.Blur()
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focus sets focus.
func (m *Model) Focus() {
	m.focused = true
	m.focusActiveField()
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
	m.displayName.Blur()
	m.name.Blur()
	m.purpose.Blur()
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	inputWidth := w/2 - 8
	m.displayName.Width = inputWidth
	m.name.Width = inputWidth
	m.purpose.Width = inputWidth
}

func (m *Model) focusActiveField() {
	m.displayName.Blur()
	m.name.Blur()
	m.purpose.Blur()
	switch m.activeField {
	case fieldDisplayName:
		m.displayName.Focus()
	case fieldName:
		m.name.Focus()
	case fieldPurpose:
		m.purpose.Focus()
	}
}

// Slug generates a URL-safe channel name from a display name.
func Slug(displayName string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(displayName) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' || r == '-' {
			b.WriteRune('-')
		}
	}
	s := b.String()
	s = strings.Trim(s, "-")
	// Collapse consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if len(s) > 64 {
		s = s[:64]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// ValidateName checks if a channel name is valid.
func ValidateName(name string) bool {
	return nameRegex.MatchString(name)
}

func (m Model) validate() string {
	dn := strings.TrimSpace(m.displayName.Value())
	if dn == "" {
		return "Display name is required"
	}
	if len(dn) > 64 {
		return "Display name max 64 chars"
	}
	nm := m.name.Value()
	if nm == "" {
		return "Channel name is required"
	}
	if !ValidateName(nm) {
		return "Invalid name: lowercase alphanumeric and hyphens, 2-64 chars"
	}
	if len(m.purpose.Value()) > 250 {
		return "Purpose max 250 chars"
	}
	return ""
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible || !m.focused {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyEscape:
		m.Close()
		return m, nil

	case tea.KeyTab:
		m.activeField = (m.activeField + 1) % fieldCount
		if m.activeField == fieldName {
			m.autoSlug = false
		}
		m.focusActiveField()
		return m, nil

	case tea.KeyShiftTab:
		m.activeField = (m.activeField - 1 + fieldCount) % fieldCount
		if m.activeField == fieldName {
			m.autoSlug = false
		}
		m.focusActiveField()
		return m, nil

	case tea.KeyLeft:
		if m.activeField == fieldType {
			m.channelType = model.ChannelOpen
			return m, nil
		}

	case tea.KeyRight:
		if m.activeField == fieldType {
			m.channelType = model.ChannelPrivate
			return m, nil
		}

	case tea.KeyEnter:
		if errMsg := m.validate(); errMsg != "" {
			m.validErr = errMsg
			return m, nil
		}
		ch := &model.Channel{
			TeamID:      m.teamID,
			Name:        m.name.Value(),
			DisplayName: strings.TrimSpace(m.displayName.Value()),
			Purpose:     strings.TrimSpace(m.purpose.Value()),
			Type:        m.channelType,
		}
		m.Close()
		return m, func() tea.Msg { return ChannelSubmittedMsg{Channel: ch} }
	}

	// Update the active text input
	var cmd tea.Cmd
	switch m.activeField {
	case fieldDisplayName:
		m.displayName, cmd = m.displayName.Update(msg)
		if m.autoSlug {
			m.name.SetValue(Slug(m.displayName.Value()))
		}
	case fieldName:
		m.name, cmd = m.name.Update(msg)
	case fieldPurpose:
		m.purpose, cmd = m.purpose.Update(msg)
	}
	m.validErr = ""

	return m, cmd
}

// View renders the channel creator overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	var items []string
	items = append(items, m.styles.ListItemActive.Render("Create Channel"))
	items = append(items, "")

	// Display Name
	label := "Display Name"
	if m.activeField == fieldDisplayName {
		label = m.styles.ListItemActive.Render("> " + label)
	} else {
		label = m.styles.ListItem.Render("  " + label)
	}
	items = append(items, label)
	items = append(items, "  "+m.displayName.View())
	items = append(items, "")

	// Channel Name
	label = "Channel Name"
	if m.activeField == fieldName {
		label = m.styles.ListItemActive.Render("> " + label)
	} else {
		label = m.styles.ListItem.Render("  " + label)
	}
	items = append(items, label)
	items = append(items, "  "+m.name.View())
	items = append(items, "")

	// Type toggle
	label = "Type"
	if m.activeField == fieldType {
		label = m.styles.ListItemActive.Render("> " + label)
	} else {
		label = m.styles.ListItem.Render("  " + label)
	}
	var openLabel, privateLabel string
	if m.channelType == model.ChannelOpen {
		openLabel = m.styles.ListItemActive.Render("[Open]")
		privateLabel = m.styles.ListItem.Render(" Private ")
	} else {
		openLabel = m.styles.ListItem.Render(" Open ")
		privateLabel = m.styles.ListItemActive.Render("[Private]")
	}
	items = append(items, label)
	items = append(items, "  "+openLabel+"  "+privateLabel)
	items = append(items, "")

	// Purpose
	label = "Purpose"
	if m.activeField == fieldPurpose {
		label = m.styles.ListItemActive.Render("> " + label)
	} else {
		label = m.styles.ListItem.Render("  " + label)
	}
	items = append(items, label)
	items = append(items, "  "+m.purpose.View())
	items = append(items, "")

	// Validation error
	if m.validErr != "" {
		items = append(items, m.styles.ErrorText.Render("  "+m.validErr))
	}

	items = append(items, m.styles.Timestamp.Render("  Enter: submit  Esc: cancel  Tab: next field"))

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}
