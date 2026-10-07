package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// View renders the full UI.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.appState == AppStateLogin || m.appState == AppStateReLogin {
		return m.loginModel.View()
	}

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

// overlayRef is one floating overlay's hooks: visibility, key routing,
// rendering, and lifecycle plumbing. Every overlay consumer (key intercepts,
// View compositing, blur/resize/restyle, mouse hit-testing) iterates the
// single overlays() table, so adding an overlay means adding one entry here.
type overlayRef struct {
	visible    func() bool
	update     func(tea.KeyMsg) tea.Cmd
	view       func() string
	blur       func()
	setSize    func(w, h int)
	setStyles  func(styles.Styles)
	closeFocus FocusArea // focus target after the overlay self-closes
}

// overlays lists the centered floating overlays in z-order (later entries
// render on top; earlier entries win key interception, though only one
// overlay is ever open at a time).
func (m *Model) overlays() []overlayRef {
	// The tag picker acts on the post under the cursor or the open thread's
	// root, so it returns to whichever pane that was. The history pane is
	// hidden behind an open thread, and keys sent there act on posts the
	// reader cannot see.
	tagPickerReturn := FocusViewport
	if m.mainPane == paneThread {
		tagPickerReturn = FocusThread
	}
	return []overlayRef{
		{
			visible: func() bool { return m.dmPicker.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.dmPicker, cmd = m.dmPicker.Update(msg)
				return cmd
			},
			view:       func() string { return m.dmPicker.View() },
			blur:       m.dmPicker.Blur,
			setSize:    m.dmPicker.SetSize,
			setStyles:  m.dmPicker.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.threadInbox.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.threadInbox, cmd = m.threadInbox.Update(msg)
				return cmd
			},
			view:       func() string { return m.threadInbox.View() },
			blur:       m.threadInbox.Blur,
			setSize:    m.threadInbox.SetSize,
			setStyles:  m.threadInbox.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.skinPicker.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.skinPicker, cmd = m.skinPicker.Update(msg)
				return cmd
			},
			view:       func() string { return m.skinPicker.View() },
			blur:       m.skinPicker.Blur,
			setSize:    m.skinPicker.SetSize,
			setStyles:  m.skinPicker.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.chCreator.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.chCreator, cmd = m.chCreator.Update(msg)
				return cmd
			},
			view:       func() string { return m.chCreator.View() },
			blur:       m.chCreator.Blur,
			setSize:    m.chCreator.SetSize,
			setStyles:  m.chCreator.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.tagPicker.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.tagPicker, cmd = m.tagPicker.Update(msg)
				return cmd
			},
			view:       func() string { return m.tagPicker.View() },
			blur:       m.tagPicker.Blur,
			setSize:    m.tagPicker.SetSize,
			setStyles:  m.tagPicker.SetStyles,
			closeFocus: tagPickerReturn,
		},
		{
			visible: func() bool { return m.palette.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.palette, cmd = m.palette.Update(msg)
				return cmd
			},
			view:       func() string { return m.palette.View() },
			blur:       m.palette.Blur,
			setSize:    m.palette.SetSize,
			setStyles:  m.palette.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.help.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.help, cmd = m.help.Update(msg)
				return cmd
			},
			view:       func() string { return m.help.View() },
			blur:       func() {},
			setSize:    m.help.SetSize,
			setStyles:  m.help.SetStyles,
			closeFocus: focusKeep, // help never takes focus
		},
	}
}

// syncActionBar pushes the current model state into the action bar before it
// is rendered or hit-tested.
func (m *Model) syncActionBar() {
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
	return (m.width - lipgloss.Width(view)) / 2, overlayY
}

func (m *Model) resizeComponents() {
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

	for i, fgLine := range fgLines {
		bgIdx := y + i
		if bgIdx < 0 || bgIdx >= len(bgLines) {
			continue
		}
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
	return lines
}

// handleThemeChosen restyles everything with the chosen theme and saves the
// choice.
func (m Model) handleThemeChosen(msg skinpicker.SkinSelectedMsg) (tea.Model, tea.Cmd) {
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
