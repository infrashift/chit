package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// Runtime themes (ADR-0016): one restyle path, the picker preview,
// /theme <name>, /theme reload, the theme-file watch and the terminal
// colors.

const (
	// themeWatchEvery is how often the watch reads the mod time of the
	// active theme file. One stat for each tick.
	themeWatchEvery = 2 * time.Second
	// maxThemeHints is the most theme names that an unknown name suggests.
	maxThemeHints = 5
)

// themeState is enough to put a theme back without a file read.
type themeState struct {
	key    string
	path   string
	mod    time.Time
	styles styles.Styles
}

// themeWatchMsg is one tick of the theme-file watch loop seq.
type themeWatchMsg struct {
	seq uint64
}

// SetActiveTheme records the theme that main resolved, so a reload, a watch
// and a preview undo know it.
func (m *Model) SetActiveTheme(t theme.Theme) {
	assert.That(t.Key != "", "SetActiveTheme: theme without a key")
	m.themeKey, m.themePath = t.Key, t.Path
	m.themeMod = modTime(t.Path)
	assert.That(m.themePath == "" || strings.HasSuffix(m.themePath, ".toml") || strings.HasSuffix(m.themePath, ".json"), "SetActiveTheme: theme file of an unknown type")
}

// restyle gives the styles to each part of the screen. It is the one
// restyle path: a pick, a preview, an undo, a reload and the watch use it.
// The returned command keeps the cursor of the input blinking.
func (m *Model) restyle(s styles.Styles) tea.Cmd {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "restyle: styles not built by styles.New")
	m.styles = s
	m.viewport.SetStyles(s)
	blink := m.input.SetStyles(s)
	m.thread.SetStyles(s)
	m.mention.SetStyles(s)
	m.actionBar.SetStyles(s)
	m.loginModel.SetStyles(s)
	m.confirm.SetStyles(s)
	m.activity.SetStyles(s)
	m.chanChips.SetStyles(s)
	m.attachTray.SetStyles(s)
	for _, o := range m.overlays() {
		o.setStyles(s)
	}
	assert.That(m.styles.TermBackground == s.TermBackground, "restyle: styles not kept")
	return tea.Batch(blink, m.paintCmd())
}

// applyTheme makes t the active theme, without a save.
func (m *Model) applyTheme(t theme.Theme) tea.Cmd {
	assert.That(t.Name != "", "applyTheme: theme without a name")
	assert.That(t.Key != "", "applyTheme: theme without a key")
	cmd := m.restyle(styles.New(t))
	pathChanged := t.Path != m.themePath
	m.SetActiveTheme(t)
	if !pathChanged {
		return cmd
	}
	return tea.Batch(cmd, m.restartThemeWatch())
}

// resolveTheme returns the theme name, from the cache of the picker
// session when it is there.
func (m *Model) resolveTheme(name string) (theme.Theme, error) {
	assert.That(name != "", "resolveTheme: empty name")
	if t, ok := m.themeCache[name]; ok {
		return t, nil
	}
	t, _, err := theme.ResolveNamed(name, config.ThemesDir())
	if err != nil {
		return theme.Theme{}, err
	}
	if m.themeCache != nil {
		m.themeCache[name] = t
	}
	assert.That(t.Key != "", "resolveTheme: theme without a key")
	return t, nil
}

// openThemePicker opens the picker on the active theme and starts a picker
// session.
func (m *Model) openThemePicker() tea.Cmd {
	assert.That(m.appState == AppStateRunning, "openThemePicker: not signed in")
	m.themeCache = make(map[string]theme.Theme)
	m.previewFrom = nil
	m.skinPicker.SetSkins(theme.ListAvailable(config.ThemesDir()))
	cmd := m.setFocus(FocusSkinPicker)
	m.skinPicker.Open(m.themeKey)
	assert.That(m.focus == FocusSkinPicker, "openThemePicker: picker not focused")
	return cmd
}

// endThemeSession forgets the preview and the cache of the picker session.
func (m *Model) endThemeSession() {
	assert.That(m != nil, "endThemeSession: nil model")
	m.previewFrom, m.themeCache = nil, nil
	assert.That(m.previewFrom == nil, "endThemeSession: preview kept")
}

// handleThemeChosen restyles everything with the chosen theme and saves the
// choice.
func (m Model) handleThemeChosen(msg skinpicker.SkinSelectedMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.Name != "", "handleThemeChosen: empty theme name")
	assert.That(m.appState == AppStateRunning, "handleThemeChosen: not signed in")
	t, err := m.resolveTheme(msg.Name)
	m.endThemeSession()
	if err != nil {
		// The name came from a list this component built, so a failure
		// here means the file changed underneath us.
		errCmd := m.setError(fmt.Errorf("load theme %q: %w", msg.Name, err))
		return m, tea.Batch(m.setFocus(FocusInput), errCmd)
	}
	cmd := m.applyTheme(t)
	return m, tea.Batch(cmd, m.saveTheme(t.Key), m.setFocus(FocusInput))
}

// saveTheme saves name under the theme setting. A failed save is shown, and
// the theme stays for this session.
func (m *Model) saveTheme(name string) tea.Cmd {
	assert.That(name != "", "saveTheme: empty name")
	assert.That(m.themeSetting != "", "saveTheme: no setting to save the theme under")
	if err := config.SaveThemeSetting(m.themeSetting, name); err != nil {
		return m.setError(fmt.Errorf("theme applied but not saved: %w", err))
	}
	return nil
}

// handleThemePreview shows a theme while the picker is open, and keeps the
// theme from before the first preview for an undo.
func (m Model) handleThemePreview(msg skinpicker.SkinPreviewMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.Name != "", "handleThemePreview: empty theme name")
	if m.previewFrom == nil {
		m.previewFrom = &themeState{key: m.themeKey, path: m.themePath, mod: m.themeMod, styles: m.styles}
	}
	t, err := m.resolveTheme(msg.Name)
	if err != nil {
		return m, m.setError(fmt.Errorf("load theme %q: %w", msg.Name, err))
	}
	assert.That(m.previewFrom != nil, "handleThemePreview: no theme to undo to")
	return m, m.applyTheme(t)
}

// handleThemeCancelled puts back the theme from before the previews.
func (m Model) handleThemeCancelled() (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleThemeCancelled: not signed in")
	from := m.previewFrom
	m.endThemeSession()
	focus := m.setFocus(FocusInput)
	if from == nil {
		return m, focus
	}
	cmd := m.restyle(from.styles)
	pathChanged := from.path != m.themePath
	m.themeKey, m.themePath, m.themeMod = from.key, from.path, from.mod
	if pathChanged {
		cmd = tea.Batch(cmd, m.restartThemeWatch())
	}
	assert.That(m.previewFrom == nil, "handleThemeCancelled: preview kept")
	return m, tea.Batch(cmd, focus)
}

// themeCommand runs /theme: no argument opens the picker, "reload" reads
// the active theme again, and a name applies and saves that theme.
func (m Model) themeCommand(args string) (tea.Model, tea.Cmd) {
	assert.That(len(args) < 4096, "themeCommand: argument longer than a message")
	assert.That(m.appState == AppStateRunning, "themeCommand: not signed in")
	name := strings.ToLower(strings.TrimSpace(args))
	switch name {
	case "":
		return m, m.openThemePicker()
	case "reload":
		return m.reloadTheme()
	}
	m.themeCache = nil
	t, err := m.resolveTheme(name)
	if err != nil {
		return m, m.setError(unknownTheme(name, err))
	}
	cmd := m.applyTheme(t)
	return m, tea.Batch(cmd, m.saveTheme(t.Key))
}

// reloadTheme reads the active theme from its file again.
func (m Model) reloadTheme() (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "reloadTheme: nil request scope")
	assert.That(m.appState == AppStateRunning, "reloadTheme: not signed in")
	if m.themeKey == "" {
		return m, m.setError(errors.New("no theme to reload"))
	}
	m.themeCache = nil
	t, err := m.resolveTheme(m.themeKey)
	if err != nil {
		return m, m.setError(fmt.Errorf("reload theme %q: %w", m.themeKey, err))
	}
	return m, m.applyTheme(t)
}

// unknownTheme explains why name did not load, with the theme names that
// hold it, when it names no theme at all.
func unknownTheme(name string, err error) error {
	assert.That(name == strings.ToLower(name), "unknownTheme: name not lowercased")
	assert.That(name != "" && err != nil, "unknownTheme: empty name or nil error")
	if !errors.Is(err, theme.ErrLocalThemeNotFound) {
		return fmt.Errorf("theme %q: %w", name, err)
	}
	var hints []string
	for _, n := range theme.ListAvailable(config.ThemesDir()) {
		if len(hints) < maxThemeHints && (strings.Contains(n, name) || strings.Contains(name, n)) {
			hints = append(hints, n)
		}
	}
	if len(hints) == 0 {
		return fmt.Errorf("no theme %q: /theme lists the themes", name)
	}
	return fmt.Errorf("no theme %q: try %s", name, strings.Join(hints, ", "))
}

// restartThemeWatch starts a new watch loop and ends the old one at its
// next tick. It returns nil when nothing is to be watched.
func (m *Model) restartThemeWatch() tea.Cmd {
	assert.That(m != nil, "restartThemeWatch: nil model")
	m.watchSeq++
	assert.That(m.watchSeq > 0, "restartThemeWatch: sequence wrapped")
	return m.watchTick(m.watchSeq)
}

// watchTick returns the next tick of loop seq, or nil when the watch is
// off or the active theme has no file.
func (m Model) watchTick(seq uint64) tea.Cmd {
	assert.That(seq <= m.watchSeq, "watchTick: a loop from the future")
	if !m.themeWatch || m.themePath == "" {
		return nil
	}
	assert.That(seq == m.watchSeq, "watchTick: a tick for an old loop")
	return tea.Tick(themeWatchEvery, func(time.Time) tea.Msg { return themeWatchMsg{seq: seq} })
}

// handleThemeWatch reloads the active theme file when its mod time
// changed. A file that does not load shows its error, and the theme stays.
func (m Model) handleThemeWatch(msg themeWatchMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.seq <= m.watchSeq, "handleThemeWatch: a tick from the future")
	if msg.seq != m.watchSeq || m.themePath == "" || !m.themeWatch {
		return m, nil
	}
	next := m.watchTick(msg.seq)
	mod := modTime(m.themePath)
	if mod.IsZero() || mod.Equal(m.themeMod) {
		return m, next
	}
	m.themeMod = mod
	t, _, err := theme.ResolveNamed(m.themeKey, config.ThemesDir())
	if err != nil {
		return m, tea.Batch(next, m.setError(fmt.Errorf("theme file %s: %w", m.themePath, err)))
	}
	assert.That(t.Path == m.themePath, "handleThemeWatch: the theme moved to another file")
	return m, tea.Batch(next, m.applyTheme(t))
}

// modTime returns the mod time of path, or the zero time when path is ""
// or cannot be read.
func modTime(path string) time.Time {
	assert.That(path == "" || strings.HasSuffix(path, ".toml") || strings.HasSuffix(path, ".json"), "modTime: not a theme file")
	if path == "" {
		return time.Time{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// paintCmd sets the terminal's default colors to the theme's, when
// theme_paint is on.
func (m Model) paintCmd() tea.Cmd {
	assert.That(len(m.styles.TermBackground) == 0 || len(m.styles.TermBackground) == 7, "paintCmd: background not #rrggbb")
	if !m.themePaint || m.clipboardOut == nil {
		return nil
	}
	seq := TerminalColors(m.styles.TermForeground, m.styles.TermBackground)
	out := m.clipboardOut
	assert.That(strings.HasPrefix(seq, "\x1b]1"), "paintCmd: not an OSC sequence")
	return func() tea.Msg {
		writeTerminal(out, seq)
		return nil
	}
}

// TerminalColors returns the OSC 10 and OSC 11 sequences that set the
// terminal's default foreground and background. An empty color resets that
// color to the terminal's own (OSC 110, OSC 111). A cell that nothing draws
// on, and each cell after a reset, shows these colors.
func TerminalColors(fg, bg string) string {
	assert.That(len(fg) == 0 || len(fg) == 7, "TerminalColors: foreground not #rrggbb")
	var b strings.Builder
	if fg == "" {
		b.WriteString("\x1b]110\x1b\\")
	} else {
		b.WriteString("\x1b]10;" + fg + "\x1b\\")
	}
	if bg == "" {
		b.WriteString("\x1b]111\x1b\\")
	} else {
		b.WriteString("\x1b]11;" + bg + "\x1b\\")
	}
	out := b.String()
	assert.That(strings.Count(out, "\x1b]") == 2, "TerminalColors: not two sequences")
	return out
}

// ResetTerminalColors gives the terminal its own default colors back. main
// calls it when the program ends.
func ResetTerminalColors(out io.Writer) {
	assert.That(out != nil, "ResetTerminalColors: nil writer")
	writeTerminal(out, TerminalColors("", ""))
}

// writeTerminal writes a control sequence straight to the terminal, as the
// clipboard does: it is not content, so it skips the renderer. A failed
// write has no channel to report on.
func writeTerminal(out io.Writer, seq string) {
	assert.That(strings.HasPrefix(seq, "\x1b]"), "writeTerminal: not an OSC sequence")
	assert.That(out != nil && seq != "", "writeTerminal: nil writer or empty sequence")
	if _, err := io.WriteString(out, seq); err != nil {
		errlog.Debug(err, "write terminal color sequence")
	}
}
