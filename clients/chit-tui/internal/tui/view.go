package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// View renders the full UI.
func (m Model) View() string {
	assert.That(m.width >= 0, "View: negative width")
	assert.That(m.height >= 0, "View: negative height")
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.appState == AppStateLogin || m.appState == AppStateReLogin {
		return m.loginModel.View()
	}
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "View: unknown main pane")
	assert.That(m.appState == AppStateRunning, "View: not signed in")

	top := m.viewport.View()
	if m.mainPane == paneThread {
		top = m.thread.View()
	}
	layout := lipgloss.JoinVertical(lipgloss.Left,
		top,
		m.input.View(),
	)

	// Composite the centered floating overlays.
	for _, o := range m.overlays() {
		if !o.visible() {
			continue
		}
		view := o.view()
		x := (m.width - lipgloss.Width(view)) / 2
		layout = placeOverlay(x, overlayY, view, layout)
	}

	// The mention autocomplete is anchored above the input box instead.
	if m.mention.Visible() {
		mentionView := m.mention.View()
		mentionHeight := lipgloss.Height(mentionView)
		mentionY := m.height - inputHeight - mentionHeight - 1
		if mentionY < 0 {
			mentionY = 0
		}
		assert.That(mentionY < m.height, "View: mention popup below the screen")
		layout = placeOverlay(1, mentionY, mentionView, layout)
	}

	// Action/status bar (m is a value receiver, so these mutations are local
	// to this render).
	m.syncActionBar()
	return lipgloss.JoinVertical(lipgloss.Left, layout, m.actionBar.View())
}

// overlayY is the row where centered floating overlays are anchored.
const overlayY = 2

// focusKeep, used as an overlayRef closeFocus, leaves focus untouched when
// the overlay closes (for overlays that never take focus, like help).
const focusKeep FocusArea = -1

// overlayID names one floating overlay. It selects the overlay's Update in
// updateOverlay, the one hook that cannot go through overlayModel because each
// sub-model's Update returns its own concrete type.
type overlayID int

const (
	overlayDMPicker overlayID = iota
	overlayThreadInbox
	overlaySkinPicker
	overlayChannelCreator
	overlayTagPicker
	overlayPalette
	overlayHelp
)

// overlayModel is what every floating overlay's sub-model provides. The
// overlays() table holds pointers to the Model's own fields, so calls through
// it read and change the Model in place.
type overlayModel interface {
	Visible() bool
	View() string
	SetSize(w, h int)
	SetStyles(s styles.Styles)
}

// blurrer is an overlay that can hold focus; overlays without Blur (help)
// never take focus and have nothing to release.
type blurrer interface {
	Blur()
}

// overlayRef is one floating overlay: its sub-model, the ID that routes keys
// to it, and where focus goes when it closes. Every overlay consumer (key
// intercepts, View compositing, blur/resize/restyle, mouse hit-testing)
// iterates the single overlays() table, so adding an overlay means adding one
// entry there and one case in updateOverlay.
type overlayRef struct {
	id         overlayID
	model      overlayModel
	closeFocus FocusArea // focus target after the overlay self-closes
}

func (o overlayRef) visible() bool {
	assert.That(o.model != nil, "overlayRef.visible: nil overlay")
	return o.model.Visible()
}

func (o overlayRef) view() string {
	assert.That(o.model != nil, "overlayRef.view: nil overlay")
	assert.That(o.model.Visible(), "overlayRef.view: rendering a closed overlay")
	return o.model.View()
}

func (o overlayRef) setSize(w, h int) {
	assert.That(o.model != nil, "overlayRef.setSize: nil overlay")
	assert.That(w >= 0, "overlayRef.setSize: negative width")
	assert.That(h >= 0, "overlayRef.setSize: negative height")
	o.model.SetSize(w, h)
}

func (o overlayRef) setStyles(newStyles styles.Styles) {
	assert.That(o.model != nil, "overlayRef.setStyles: nil overlay")
	o.model.SetStyles(newStyles)
}

// blur releases the overlay's focus, if it is one that can hold focus.
func (o overlayRef) blur() {
	assert.That(o.model != nil, "overlayRef.blur: nil overlay")
	if b, ok := o.model.(blurrer); ok {
		b.Blur()
	}
}

// overlays lists the centered floating overlays in z-order (later entries
// render on top; earlier entries win key interception, though only one
// overlay is ever open at a time).
func (m *Model) overlays() []overlayRef {
	assert.That(m != nil, "overlays: nil model")
	// The tag picker acts on the post under the cursor or the open thread's
	// root, so it returns to whichever pane that was. The history pane is
	// hidden behind an open thread, and keys sent there act on posts the
	// reader cannot see.
	tagPickerReturn := FocusViewport
	if m.mainPane == paneThread {
		tagPickerReturn = FocusThread
	}
	refs := []overlayRef{
		{id: overlayDMPicker, model: &m.dmPicker, closeFocus: FocusInput},
		{id: overlayThreadInbox, model: &m.threadInbox, closeFocus: FocusInput},
		{id: overlaySkinPicker, model: &m.skinPicker, closeFocus: FocusInput},
		{id: overlayChannelCreator, model: &m.chCreator, closeFocus: FocusInput},
		{id: overlayTagPicker, model: &m.tagPicker, closeFocus: tagPickerReturn},
		{id: overlayPalette, model: &m.palette, closeFocus: FocusInput},
		{id: overlayHelp, model: &m.help, closeFocus: focusKeep}, // help never takes focus
	}
	assert.That(len(refs) == int(overlayHelp)+1, "overlays: an overlay is missing from the table")
	assert.That(tagPickerReturn == FocusViewport || tagPickerReturn == FocusThread, "overlays: tag picker returns to no pane")
	return refs
}

// updateOverlay routes a key to the overlay id names and stores its new state
// in the Model.
func (m *Model) updateOverlay(id overlayID, msg tea.KeyMsg) tea.Cmd {
	assert.That(m != nil, "updateOverlay: nil model")
	assert.That(id >= overlayDMPicker, "updateOverlay: overlay ID before the first")
	assert.That(id <= overlayHelp, "updateOverlay: overlay ID past the last")
	var cmd tea.Cmd
	switch id {
	case overlayDMPicker:
		m.dmPicker, cmd = m.dmPicker.Update(msg)
	case overlayThreadInbox:
		m.threadInbox, cmd = m.threadInbox.Update(msg)
	case overlaySkinPicker:
		m.skinPicker, cmd = m.skinPicker.Update(msg)
	case overlayChannelCreator:
		m.chCreator, cmd = m.chCreator.Update(msg)
	case overlayTagPicker:
		m.tagPicker, cmd = m.tagPicker.Update(msg)
	case overlayPalette:
		m.palette, cmd = m.palette.Update(msg)
	case overlayHelp:
		m.help, cmd = m.help.Update(msg)
	}
	return cmd
}

// syncActionBar pushes the current model state into the action bar before it
// is rendered or hit-tested.
func (m *Model) syncActionBar() {
	assert.That(m != nil, "syncActionBar: nil model")
	assert.That(m.focus >= FocusViewport, "syncActionBar: focus before the first area")
	assert.That(m.focus <= FocusThreadInbox, "syncActionBar: focus past the last area")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "syncActionBar: unknown main pane")
	team := ""
	if m.activeTeam != nil {
		team = m.activeTeam.DisplayName
	}
	user := ""
	if m.me != nil {
		user = m.me.Username
	}
	errStr := ""
	if m.err != nil {
		errStr = m.err.Error()
	}
	m.actionBar.SetContext(team, m.activeChannelDisplayName(), user)
	m.actionBar.SetError(errStr)
	m.actionBar.SetConnected(m.wsConnected)
	m.actionBar.SetThreadOpen(m.mainPane == paneThread)
	m.actionBar.SetEditing(m.editingPostID != "")

	// Replying needs a focused history pane with a post under the cursor;
	// the button appears only then, which is also the hint that the pane has
	// to be focused first.
	m.actionBar.SetCanReply(m.mainPane == paneChannel &&
		m.focus == FocusViewport && m.viewport.SelectedPost() != nil)
}

// paletteOrigin returns the screen position of the palette overlay. It is the
// single source of truth shared by View and mouse hit-testing.
func (m Model) paletteOrigin(view string) (int, int) {
	assert.That(m.width >= 0, "paletteOrigin: negative width")
	assert.That(m.palette.Visible(), "paletteOrigin: palette not open")
	return (m.width - lipgloss.Width(view)) / 2, overlayY
}

func (m *Model) resizeComponents() {
	assert.That(m != nil, "resizeComponents: nil model")
	assert.That(m.width >= 0, "resizeComponents: negative width")
	assert.That(m.height >= 0, "resizeComponents: negative height")
	vpHeight := m.height - inputHeight - 1 // -1 for the action bar

	m.viewport.SetSize(m.width, vpHeight)
	m.input.SetSize(m.width, inputHeight)
	m.actionBar.SetSize(m.width)
	m.thread.SetSize(m.width, vpHeight)
	for _, o := range m.overlays() {
		o.setSize(m.width, m.height)
	}
}

// placeOverlay places a foreground string on top of a background string at x, y.
// It is ANSI-aware: it uses charmbracelet/x/ansi to truncate styled lines
// without breaking escape sequences.
func placeOverlay(x, y int, fg, bg string) string {
	bgLines := splitLines(bg)
	fgLines := splitLines(fg)
	assert.That(len(bgLines) >= 1, "placeOverlay: background has no lines")
	assert.That(len(fgLines) >= 1, "placeOverlay: overlay has no lines")

	for i, fgLine := range fgLines {
		bgIdx := y + i
		if bgIdx < 0 || bgIdx >= len(bgLines) {
			continue
		}
		assert.That(bgIdx < len(bgLines), "placeOverlay: row past the background")
		bgLine := bgLines[bgIdx]
		fgWidth := ansi.StringWidth(fgLine)
		bgWidth := ansi.StringWidth(bgLine)

		// Part of background before the overlay
		var before string
		if bgWidth >= x {
			before = ansi.Truncate(bgLine, x, "")
		} else {
			before = bgLine + strings.Repeat(" ", x-bgWidth)
		}

		// Part of background after the overlay
		afterStart := x + fgWidth
		var after string
		if bgWidth > afterStart {
			after = ansi.TruncateLeft(bgLine, afterStart, "")
		}

		bgLines[bgIdx] = before + fgLine + after
	}

	return strings.Join(bgLines, "\n")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	assert.That(len(lines) >= 1, "splitLines: no lines")
	assert.That(start <= len(s), "splitLines: line start past the end")
	return lines
}

// handleThemeChosen restyles everything with the chosen theme and saves the
// choice.
func (m Model) handleThemeChosen(msg skinpicker.SkinSelectedMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.Name != "", "handleThemeChosen: empty theme name")
	assert.That(m.appState == AppStateRunning, "handleThemeChosen: not signed in")
	assert.That(m.themeSetting != "", "handleThemeChosen: no setting to save the theme under")
	t, _, err := theme.ResolveNamed(msg.Name, config.ThemesDir())
	if err != nil {
		// The name came from a list this component built, so a failure
		// here means the file changed underneath us.
		errCmd := m.setError(fmt.Errorf("load theme %q: %w", msg.Name, err))
		return m, tea.Batch(m.setFocus(FocusInput), errCmd)
	}

	// Persist so the choice survives a restart. Failing to write is worth
	// surfacing but must not undo the theme change for this session.
	var saveCmd tea.Cmd
	if err := config.SaveThemeSetting(m.themeSetting, msg.Name); err != nil {
		saveCmd = m.setError(fmt.Errorf("theme applied but not saved: %w", err))
	}

	newStyles := styles.New(t)
	m.styles = newStyles
	m.viewport.SetStyles(newStyles)
	m.input.SetStyles(newStyles)
	m.thread.SetStyles(newStyles)
	m.mention.SetStyles(newStyles)
	m.actionBar.SetStyles(newStyles)
	m.loginModel.SetStyles(newStyles)
	for _, o := range m.overlays() {
		o.setStyles(newStyles)
	}
	return m, tea.Batch(m.setFocus(FocusInput), saveCmd)
}
