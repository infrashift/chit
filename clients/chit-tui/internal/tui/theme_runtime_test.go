package tui_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// The picker previews the theme under the cursor, Esc puts the old theme
// back, and Enter keeps and saves the previewed one.

func down() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyDown} }

// preview opens the picker on m, moves the cursor down once and applies the
// preview. It returns the name of the previewed theme.
func preview(t *testing.T, m tui.Model) (tui.Model, string) {
	t.Helper()
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/theme"})
	m, cmd := step(t, m, down())
	msg := wantMsg[skinpicker.SkinPreviewMsg](t, cmd)
	m, _ = step(t, m, msg)
	return m, msg.Name
}

func TestThemePreview_RestylesUnderThePicker(t *testing.T) {
	trueColor(t)
	m := themedModel(t, "tokyo-night")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("draft")})
	m, name := preview(t, m)
	if name != "tokyo-night-day" {
		t.Fatalf("the picker did not open on the active theme: previewed %q", name)
	}
	if stale := staleColors(m.View(), "tokyo-night", name); stale != "" {
		t.Errorf("the preview left colors of the old theme: %s", stale)
	}
}

func TestThemePreview_EscPutsTheOldThemeBack(t *testing.T) {
	trueColor(t)
	m := themedModel(t, "tokyo-night")
	before := m.View()
	m, _ = preview(t, m)
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	m, _ = step(t, m, wantMsg[skinpicker.SkinCancelledMsg](t, cmd))
	if got := m.View(); got != before {
		t.Errorf("Esc did not put the old theme back:\n%q", firstDiff(got, before))
	}
	if data, err := os.ReadFile(config.FilePath()); err == nil && strings.Contains(string(data), "tokyo-night-day") {
		t.Errorf("a preview was saved: %s", data)
	}
}

func TestThemePreview_EnterKeepsAndSaves(t *testing.T) {
	trueColor(t)
	m := themedModel(t, "tokyo-night")
	m, name := preview(t, m)
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, wantMsg[skinpicker.SkinSelectedMsg](t, cmd))
	if stale := staleColors(m.View(), "tokyo-night", name); stale != "" {
		t.Errorf("the kept theme shows old colors: %s", stale)
	}
	data, err := os.ReadFile(config.FilePath())
	if err != nil || !strings.Contains(string(data), `"`+name+`"`) {
		t.Errorf("config = %q, %v; want %s saved", data, err, name)
	}
}

func TestThemeCommand_ByName(t *testing.T) {
	trueColor(t)
	m := themedModel(t, "tokyo-night")
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/theme Kanagawa"})
	if stale := staleColors(m.View(), "tokyo-night", "kanagawa"); stale != "" {
		t.Errorf("/theme kanagawa left old colors: %s", stale)
	}
	data, err := os.ReadFile(config.FilePath())
	if err != nil || !strings.Contains(string(data), `"kanagawa"`) {
		t.Errorf("config = %q, %v; want kanagawa saved", data, err)
	}
}

func TestThemeCommand_UnknownNameSuggests(t *testing.T) {
	m := themedModel(t, "tokyo-night")
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/theme catpp"})
	if v := viewOf(m); !strings.Contains(v, "try catppuccin, catppuccin-latte") {
		t.Errorf("no hint for a near name:\n%s", v)
	}
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/theme zzz"})
	if v := viewOf(m); !strings.Contains(v, `no theme "zzz"`) {
		t.Errorf("no error for an unknown name:\n%s", v)
	}
}

// writeLocalTheme writes a local theme file with background bg and returns
// the theme that it resolves to.
func writeLocalTheme(t *testing.T, name, bg string) theme.Theme {
	t.Helper()
	dir := config.ThemesDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "name = \"Mine\"\nbackground = \"" + bg + "\"\nforeground = \"#d0d0d0\"\n"
	for _, k := range []string{"accent", "subtle", "error", "success", "warning", "border", "active_border",
		"highlight", "muted", "username", "timestamp", "unread_badge", "pin_badge", "channel_active",
		"mention_badge", "mention_text", "mention_self_bg", "tag_badge"} {
		body += k + " = \"#5fafff\"\n"
	}
	path := filepath.Join(dir, name+".toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	th, _, err := theme.ResolveNamed(name, dir)
	if err != nil {
		t.Fatalf("resolve the local theme: %v", err)
	}
	return th
}

// localThemedModel is a model on a local theme, with the watch and the
// paint on, that writes its terminal sequences to term.
func localThemedModel(t *testing.T, th theme.Theme, term io.Writer) tui.Model {
	t.Helper()
	cfg := &config.Config{ServerURL: "http://localhost:8065", ThemeWatch: true, ThemePaint: true}
	m := tui.NewModel(cfg, &mockClient{me: &model.User{ID: "u1"}}, nil, styles.New(th), nil, nil, nil)
	m.SetClipboardOutput(term)
	m.SetActiveTheme(th)
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = step(t, m, tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	return m
}

func TestThemeCommand_Reload(t *testing.T) {
	th := writeLocalTheme(t, "reload-me", "#101010")
	var term bytes.Buffer
	m := localThemedModel(t, th, &term)
	writeLocalTheme(t, "reload-me", "#202020")
	_, cmd := step(t, m, input.SlashTriggerMsg{Input: "/theme reload"})
	drain(cmd)
	if !strings.Contains(term.String(), "\x1b]11;#202020") {
		t.Errorf("/theme reload did not apply the edited file: %q", term.String())
	}
}

func TestThemeWatch_ReloadsAnEditedFile(t *testing.T) {
	th := writeLocalTheme(t, "watch-me", "#111111")
	var term bytes.Buffer
	m := localThemedModel(t, th, &term)
	writeLocalTheme(t, "watch-me", "#222222")
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(th.Path, later, later); err != nil {
		t.Fatal(err)
	}
	m, cmd := step(t, m, tui.ThemeWatchTick(m))
	drain(cmd)
	if !strings.Contains(term.String(), "\x1b]11;#222222") {
		t.Fatalf("the watch did not apply the edited file: %q", term.String())
	}
	// A broken file shows its error, and the theme stays.
	if err := os.WriteFile(th.Path, []byte("background = 7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	even := later.Add(time.Minute)
	if err := os.Chtimes(th.Path, even, even); err != nil {
		t.Fatal(err)
	}
	m, _ = step(t, m, tui.ThemeWatchTick(m))
	if v := viewOf(m); !strings.Contains(v, "theme file") {
		t.Errorf("no error for a broken theme file:\n%s", v)
	}
}

func TestThemePaint_SetsTheTerminalColors(t *testing.T) {
	th := writeLocalTheme(t, "paint-me", "#123456")
	var term bytes.Buffer
	m := localThemedModel(t, th, &term)
	drain(m.Init())
	if !strings.Contains(term.String(), "\x1b]10;#d0d0d0\x1b\\\x1b]11;#123456\x1b\\") {
		t.Errorf("Init did not paint the terminal: %q", term.String())
	}
	term.Reset()
	tui.ResetTerminalColors(&term)
	if term.String() != "\x1b]110\x1b\\\x1b]111\x1b\\" {
		t.Errorf("ResetTerminalColors = %q", term.String())
	}
	if got := tui.TerminalColors("", "#000000"); got != "\x1b]110\x1b\\\x1b]11;#000000\x1b\\" {
		t.Errorf("TerminalColors with no foreground = %q", got)
	}
}

// Without theme_paint, the client leaves the terminal colors alone.
func TestThemePaint_Off(t *testing.T) {
	var term bytes.Buffer
	m := tui.NewModel(&config.Config{ServerURL: "http://localhost:8065"}, &mockClient{}, nil, styles.New(theme.TokyoNight()), nil, nil, nil)
	m.SetClipboardOutput(&term)
	drain(m.Init())
	if strings.Contains(term.String(), "\x1b]11") {
		t.Errorf("painted with theme_paint off: %q", term.String())
	}
}

func TestThemeKey_OpensThePicker(t *testing.T) {
	m := themedModel(t, "tokyo-night")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.Focused() != tui.FocusSkinPicker || !strings.Contains(viewOf(m), "Select Theme") {
		t.Errorf("ctrl+t did not open the theme picker:\n%s", viewOf(m))
	}
}
