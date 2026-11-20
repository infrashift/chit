package chcreator

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// nameRegex is the allowed shape of a channel name: lowercase
// letters, digits and inner dashes, 2 to 64 characters.
//
// semscope: allow(NASA-P10-06) compiled once, never changed
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

	m := Model{
		displayName: dn,
		name:        nm,
		purpose:     pp,
		channelType: model.ChannelOpen,
		autoSlug:    true,
		styles:      s,
	}
	assert.That(m.channelType == model.ChannelOpen || m.channelType == model.ChannelPrivate, "chcreator.New: unknown channel type")
	assert.That(!m.displayName.Focused() && !m.name.Focused() && !m.purpose.Focused(), "chcreator.New: a field starts focused")
	return m
}

// Open shows the channel creator overlay with the given team ID.
func (m *Model) Open(teamID string) {
	// A name the slug or the user writes must fit: the name rule allows 64.
	assert.That(m.name.CharLimit >= 64, "Model.Open: name input shorter than the longest valid name")
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
	assert.That(m.displayName.Value() == "" && m.name.Value() == "" && m.purpose.Value() == "", "Model.Open: form not cleared")
	assert.That(m.displayName.Focused(), "Model.Open: display name not focused")
	assert.That(!m.name.Focused() && !m.purpose.Focused(), "Model.Open: a later field kept focus")
}

// Close hides the channel creator overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.displayName.Blur()
	m.name.Blur()
	m.purpose.Blur()
	assert.That(!m.displayName.Focused() && !m.name.Focused() && !m.purpose.Focused(), "Model.Close: a field kept focus")
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focus sets focus.
func (m *Model) Focus() {
	assert.That(m.activeField >= 0 && m.activeField < fieldCount, "Model.Focus: active field out of range")
	m.focused = true
	m.focusActiveField()
	assert.That(m.activeField == fieldType || m.displayName.Focused() || m.name.Focused() || m.purpose.Focused(), "Model.Focus: active text field not focused")
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
	m.displayName.Blur()
	m.name.Blur()
	m.purpose.Blur()
	assert.That(!m.displayName.Focused() && !m.name.Focused() && !m.purpose.Focused(), "Model.Blur: a field kept focus")
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
	assert.That(m.activeField >= 0 && m.activeField < fieldCount, "Model.focusActiveField: active field out of range")
	assert.That(m.focused, "Model.focusActiveField: form not focused")
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
	assert.That(m.displayName.Focused() == (m.activeField == fieldDisplayName), "Model.focusActiveField: display name focus wrong")
	assert.That(m.name.Focused() == (m.activeField == fieldName), "Model.focusActiveField: channel name focus wrong")
	assert.That(m.purpose.Focused() == (m.activeField == fieldPurpose), "Model.focusActiveField: purpose focus wrong")
}

// Slug generates a URL-safe channel name from a display name.
func Slug(displayName string) string {
	var b strings.Builder
	// Names allow only ASCII letters, digits and hyphens. Other letters
	// are dropped rather than kept for the name check to reject, which
	// keeps the slug ASCII and so safe to cut by byte below.
	for _, r := range strings.ToLower(displayName) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-':
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
	assert.That(len(s) <= 64, "Slug: longer than 64 bytes")
	assert.That(!strings.Contains(s, "--"), "Slug: consecutive hyphens")
	assert.That(!strings.HasPrefix(s, "-") && !strings.HasSuffix(s, "-"), "Slug: leading or trailing hyphen")
	assert.That(strings.Trim(s, "abcdefghijklmnopqrstuvwxyz0123456789-") == "", "Slug: character outside [a-z0-9-]")
	assert.That(len(s) < 2 || ValidateName(s), "Slug: a slug of two or more characters is not a valid name")
	return s
}

// ValidateName checks if a channel name is valid.
func ValidateName(name string) bool {
	ok := nameRegex.MatchString(name)
	assert.That(!ok || (len(name) >= 2 && len(name) <= 64), "ValidateName: valid name outside 2-64 characters")
	assert.That(!ok || (name[0] != '-' && name[len(name)-1] != '-'), "ValidateName: valid name with an outer hyphen")
	return ok
}

func (m Model) validate() string {
	assert.That(utf8.RuneCountInString(m.displayName.Value()) <= m.displayName.CharLimit, "Model.validate: display name over its input limit")
	assert.That(utf8.RuneCountInString(m.purpose.Value()) <= m.purpose.CharLimit, "Model.validate: purpose over its input limit")
	dn := strings.TrimSpace(m.displayName.Value())
	if dn == "" {
		return "Display name is required"
	}
	if utf8.RuneCountInString(dn) > 64 {
		return "Display name max 64 chars"
	}
	nm := m.name.Value()
	if nm == "" {
		return "Channel name is required"
	}
	if !ValidateName(nm) {
		return "Invalid name: lowercase alphanumeric and hyphens, 2-64 chars"
	}
	if utf8.RuneCountInString(m.purpose.Value()) > 250 {
		return "Purpose max 250 chars"
	}
	return ""
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible || !m.focused {
		return m, nil
	}

	if blink, ok := msg.(cursor.BlinkMsg); ok {
		return m.blink(blink)
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	assert.That(m.activeField >= 0 && m.activeField < fieldCount, "Model.Update: active field out of range")
	assert.That(m.channelType == model.ChannelOpen || m.channelType == model.ChannelPrivate, "Model.Update: unknown channel type")

	switch keyMsg.Type {
	case tea.KeyEscape:
		m.Close()
		assert.That(!m.visible && !m.focused, "Model.Update: Esc left the form open")
		return m, nil

	case tea.KeyTab:
		m.moveField(1)
		return m, nil

	case tea.KeyShiftTab:
		m.moveField(-1)
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
		return m.submit()
	}

	return m.updateField(msg)
}

// moveField moves focus delta fields along, wrapping at either end.
func (m *Model) moveField(delta int) {
	assert.That(delta == 1 || delta == -1, "Model.moveField: step other than one field")
	m.activeField = (m.activeField + delta + fieldCount) % fieldCount
	assert.That(m.activeField >= 0 && m.activeField < fieldCount, "Model.moveField: active field out of range")
	if m.activeField == fieldName {
		m.autoSlug = false
	}
	m.focusActiveField()
}

// submit sends the channel described, unless it is invalid.
func (m Model) submit() (Model, tea.Cmd) {
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
	assert.That(ValidateName(ch.Name), "Model.submit: invalid channel name passed validation")
	assert.That(ch.DisplayName != "", "Model.submit: empty display name passed validation")
	assert.That(ch.Type == model.ChannelOpen || ch.Type == model.ChannelPrivate, "Model.submit: unknown channel type")
	assert.That(utf8.RuneCountInString(ch.Purpose) <= 250, "Model.submit: purpose over 250 characters passed validation")
	m.Close()
	assert.That(!m.visible, "Model.submit: form left open")
	return m, func() tea.Msg { return ChannelSubmittedMsg{Channel: ch} }
}

// updateField gives msg to the active text input.
// blink gives a cursor blink to the active text field. Unlike updateField,
// it keeps the validation error and the slug, because a blink is no edit.
func (m Model) blink(msg cursor.BlinkMsg) (Model, tea.Cmd) {
	assert.That(m.activeField >= 0 && m.activeField < fieldCount, "Model.blink: active field out of range")
	before := m.validErr
	var cmd tea.Cmd
	switch m.activeField {
	case fieldDisplayName:
		m.displayName, cmd = m.displayName.Update(msg)
	case fieldName:
		m.name, cmd = m.name.Update(msg)
	case fieldPurpose:
		m.purpose, cmd = m.purpose.Update(msg)
	}
	assert.That(m.validErr == before, "Model.blink: a blink changed the validation error")
	return m, cmd
}

func (m Model) updateField(msg tea.Msg) (Model, tea.Cmd) {
	assert.That(m.activeField >= 0 && m.activeField < fieldCount, "Model.updateField: active field out of range")
	var cmd tea.Cmd
	switch m.activeField {
	case fieldDisplayName:
		m.displayName, cmd = m.displayName.Update(msg)
		if m.autoSlug {
			m.name.SetValue(Slug(m.displayName.Value()))
			assert.That(m.name.Value() == Slug(m.displayName.Value()), "Model.updateField: slug cut short by the name input")
		}
	case fieldName:
		m.name, cmd = m.name.Update(msg)
	case fieldPurpose:
		m.purpose, cmd = m.purpose.Update(msg)
	}
	m.validErr = ""
	assert.That(utf8.RuneCountInString(m.name.Value()) <= m.name.CharLimit, "Model.updateField: channel name over its input limit")

	return m, cmd
}

// fieldLabel renders a field's label, marked when the field is active.
func (m Model) fieldLabel(label string, field int) string {
	assert.That(field >= 0 && field < fieldCount, "Model.fieldLabel: field out of range")
	assert.That(label != "", "Model.fieldLabel: empty label")
	if m.activeField == field {
		return m.styles.ListItemActive.Render("> " + label)
	}
	return m.styles.ListItem.Render("  " + label)
}

// typeToggle renders the Open / Private choice with the current one marked.
func (m Model) typeToggle() string {
	assert.That(m.channelType == model.ChannelOpen || m.channelType == model.ChannelPrivate, "Model.typeToggle: unknown channel type")
	if m.channelType == model.ChannelOpen {
		return m.styles.ListItemActive.Render("[Open]") + "  " + m.styles.ListItem.Render(" Private ")
	}
	return m.styles.ListItem.Render(" Open ") + "  " + m.styles.ListItemActive.Render("[Private]")
}

// View renders the channel creator overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	assert.That(m.activeField >= 0 && m.activeField < fieldCount, "Model.View: active field out of range")
	assert.That(m.validErr == "" || m.validErr == m.validate(), "Model.View: stale validation error")
	assert.That(m.channelType == model.ChannelOpen || m.channelType == model.ChannelPrivate, "Model.View: unknown channel type")
	items := []string{
		m.styles.ListItemActive.Render("Create Channel"),
		"",
		m.fieldLabel("Display Name", fieldDisplayName),
		"  " + m.displayName.View(),
		"",
		m.fieldLabel("Channel Name", fieldName),
		"  " + m.name.View(),
		"",
		m.fieldLabel("Type", fieldType),
		"  " + m.typeToggle(),
		"",
		m.fieldLabel("Purpose", fieldPurpose),
		"  " + m.purpose.View(),
		"",
	}

	// Validation error
	if m.validErr != "" {
		items = append(items, m.styles.ErrorText.Render("  "+m.validErr))
	}

	items = append(items, m.styles.Timestamp.Render("  Enter: submit  Esc: cancel  Tab: next field"))

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}
