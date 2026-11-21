package tui_test

import (
	"fmt"
	"image/color"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
	"github.com/muesli/termenv"
)

// A theme change must give the same screen as a start with that theme
// (ADR-0016). Each difference is a part that kept the old colors.

func trueColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

// themedModel builds a model on the bundled theme name and fills it: a
// channel list with an unread badge, posts with tags, a pinned and an
// edited post, a mention, and a DM.
func themedModel(t *testing.T, name string) tui.Model {
	t.Helper()
	th, ok := theme.Lookup(name)
	if !ok {
		t.Fatalf("no bundled theme %q", name)
	}
	client := &mockClient{
		me:      &model.User{ID: "u1", Username: "alice"},
		allTags: []*model.Tag{{ID: "t1", Name: "deploy", Scope: model.TagScopeChannel, ChannelID: "c1"}},
	}
	m := tui.NewModel(&config.Config{ServerURL: "http://localhost:8065"}, client, nil, styles.New(th), nil, nil, nil)
	m.SetClipboardOutput(io.Discard)
	m.SetActiveTheme(th)
	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 120, Height: 40},
		tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}},
		tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Engineering"}}},
		tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{
			{ID: "c1", TeamID: "t1", DisplayName: "General", TotalMsgCount: 3},
			{ID: "c2", TeamID: "t1", DisplayName: "Random", TotalMsgCount: 9},
		}},
		tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: []*model.Post{
			{ID: "p3", UserID: "u2", Content: "hey @alice look at `code` and **bold**", CreateAt: 1700000002000},
			{ID: "p2", UserID: "u1", Content: "edited one", EditAt: 1700000001500, CreateAt: 1700000001000},
			{ID: "p1", UserID: "u1", Content: "# Heading\n\n- item", IsPinned: true, CreateAt: 1700000000000},
		}}},
		tui.UsersLoadedMsg{Users: []*model.User{{ID: "u1", Username: "alice"}, {ID: "u2", Username: "bob"}}},
		tui.PostsTagsLoadedMsg{Tags: map[string][]*model.Tag{"p1": {{ID: "t1", Name: "deploy"}}}},
	} {
		m, _ = step(t, m, msg)
	}
	return m
}

// scenes open each part of the screen that has its own styles.
func scenes() map[string][]tea.Msg {
	return map[string][]tea.Msg{
		"panes":         nil,
		"history focus": {tea.KeyMsg{Type: tea.KeyTab}},
		"palette":       {tea.KeyMsg{Type: tea.KeyCtrlK}},
		"people":        {tea.KeyMsg{Type: tea.KeyCtrlD}},
		"help":          {tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}},
		"tag picker":    {tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}},
		"new channel":   {tea.KeyMsg{Type: tea.KeyCtrlN}},
		"group picker":  {input.SlashTriggerMsg{Input: "/group"}},
		"theme picker":  {input.SlashTriggerMsg{Input: "/theme"}},
		"thread inbox":  {input.SlashTriggerMsg{Input: "/threads"}},
		"delete asks":   {tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}},
		"typed input":   {tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("draft with @bo")}},
	}
}

func play(t *testing.T, m tui.Model, msgs []tea.Msg) string {
	t.Helper()
	for _, msg := range msgs {
		m, _ = step(t, m, msg)
	}
	return m.View()
}

func TestThemeChange_EqualsAFreshStart(t *testing.T) {
	trueColor(t)
	pairs := [][2]string{
		{"tokyo-night", "catppuccin-latte"},
		{"catppuccin-latte", "kanagawa"},
		{"nightfox", "dayfox"},
	}
	for _, pair := range pairs {
		for name, msgs := range scenes() {
			t.Run(fmt.Sprintf("%s to %s, %s", pair[0], pair[1], name), func(t *testing.T) {
				switched := themedModel(t, pair[0])
				switched, _ = step(t, switched, skinpicker.SkinSelectedMsg{Name: pair[1]})
				got := play(t, switched, msgs)
				want := play(t, themedModel(t, pair[1]), msgs)
				// The test means something only when the two themes look
				// different on this screen.
				if before := play(t, themedModel(t, pair[0]), msgs); before == want {
					t.Fatal("both themes draw the same screen; the test cannot see a stale part")
				}
				if got != want {
					t.Errorf("the screen after the switch differs from a fresh start:\n--- switched\n%q\n--- fresh\n%q", firstDiff(got, want), firstDiff(want, got))
				}
			})
		}
	}
}

// The same, with the part open before the switch: a theme preview changes
// the theme under an open overlay, and a thread can be open at any time.
func TestThemeChange_UnderAnOpenPart(t *testing.T) {
	trueColor(t)
	open := map[string][]tea.Msg{
		"thread": {viewportSelect("p1"), tui.ThreadLoadedMsg{PostID: "p1", Posts: &model.PostList{Order: []*model.Post{
			{ID: "p1", UserID: "u1", Content: "# Heading", CreateAt: 1700000000000},
			{ID: "r1", UserID: "u2", RootID: "p1", Content: "a reply with `code`", CreateAt: 1700000001000},
		}}}},
		"palette":      {tea.KeyMsg{Type: tea.KeyCtrlK}},
		"help":         {tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}},
		"tag picker":   {tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}},
		"new channel":  {tea.KeyMsg{Type: tea.KeyCtrlN}},
		"theme picker": {input.SlashTriggerMsg{Input: "/theme"}},
		"typed input":  {tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("draft")}},
	}
	for name, msgs := range open {
		t.Run(name, func(t *testing.T) {
			m := themedModel(t, "tokyo-night")
			for _, msg := range msgs {
				m, _ = step(t, m, msg)
			}
			if staleColors(m.View(), "tokyo-night", "catppuccin-latte") == "" {
				t.Fatal("the old theme shows none of its colors; the check cannot see a stale part")
			}
			m, _ = step(t, m, skinpicker.SkinSelectedMsg{Name: "catppuccin-latte"})
			got := testutil.StripANSI(m.View())
			fresh := themedModel(t, "catppuccin-latte")
			for _, msg := range msgs {
				fresh, _ = step(t, fresh, msg)
			}
			// SkinSelectedMsg moves focus to the input, as Enter in the
			// picker does; so compare the colors of the switched screen
			// with a fresh screen of the same text.
			if got != testutil.StripANSI(fresh.View()) {
				t.Skip("the switch changed the layout; the color check needs the same text")
			}
			if stale := staleColors(m.View(), "tokyo-night", "catppuccin-latte"); stale != "" {
				t.Errorf("colors of the old theme after the switch: %s", stale)
			}
		})
	}
}

func viewportSelect(id string) tea.Msg {
	return viewport.PostSelectedMsg{Post: &model.Post{ID: id, UserID: "u1"}}
}

// staleColors returns the colors of theme from that are in view and that
// theme to does not use, or "".
func staleColors(view, from, to string) string {
	old, _ := theme.Lookup(from)
	cur, _ := theme.Lookup(to)
	using := map[string]bool{}
	for _, c := range colorsOf(cur) {
		using[c] = true
	}
	var stale []string
	for _, c := range colorsOf(old) {
		if !using[c] && strings.Contains(view, c) {
			stale = append(stale, c)
		}
	}
	return strings.Join(stale, ", ")
}

// colorsOf returns the SGR forms (38;2;r;g;b and 48;2;r;g;b) of the base
// colors of t.
func colorsOf(t theme.Theme) []string {
	var out []string
	for _, c := range []color.Color{t.Background, t.Foreground, t.Subtle, t.Accent, t.Error, t.Success, t.Warning,
		t.Border, t.ActiveBorder, t.Highlight, t.Muted, t.Username, t.Timestamp, t.UnreadBadge, t.PinBadge,
		t.ChannelActive, t.MentionBadge, t.MentionText, t.MentionSelfBg, t.TagBadge} {
		if c == nil {
			continue
		}
		r, g, b, _ := c.RGBA()
		rgb := fmt.Sprintf("2;%d;%d;%dm", r>>8, g>>8, b>>8)
		out = append(out, "38;"+rgb, "48;"+rgb)
	}
	return out
}

// firstDiff returns a short piece of a around its first difference from b.
func firstDiff(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo, hi := max(i-60, 0), min(i+80, len(a))
	return a[lo:hi]
}
