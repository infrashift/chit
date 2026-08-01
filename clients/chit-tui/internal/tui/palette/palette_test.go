package palette_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func newPalette() palette.Model {
	m := palette.New(styles.New(theme.TokyoNight()))
	m.SetSize(120, 40)
	return m
}

func typeRunes(m palette.Model, s string) (palette.Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, r := range s {
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m, cmd
}

func testChannels() []*model.Channel {
	return []*model.Channel{
		{ID: "c1", DisplayName: "General", TeamID: "t1", LastPostAt: 100},
		{ID: "c2", DisplayName: "Incidents", TeamID: "t1", LastPostAt: 300},
		{ID: "c3", DisplayName: "Random", TeamID: "t1", LastPostAt: 200},
	}
}

func TestPalette_ModeDetection(t *testing.T) {
	m := newPalette()
	m.Open("")

	cases := []struct {
		typed string
		want  palette.Mode
	}{
		{"gen", palette.ModeChannels},
		{"", palette.ModeChannels},
	}
	for _, c := range cases {
		p := m
		p, _ = typeRunes(p, c.typed)
		if got := p.Mode(); got != c.want {
			t.Errorf("after typing %q mode = %v, want %v", c.typed, got, c.want)
		}
	}

	for prefix, want := range map[string]palette.Mode{
		"@": palette.ModeUsers,
		"/": palette.ModeCommands,
		"?": palette.ModeSearch,
	} {
		p := m
		p, _ = typeRunes(p, prefix)
		if got := p.Mode(); got != want {
			t.Errorf("after typing %q mode = %v, want %v", prefix, got, want)
		}
	}
}

func TestPalette_OpenWithPrefixSetsMode(t *testing.T) {
	m := newPalette()
	m.Open("?")
	if got := m.Mode(); got != palette.ModeSearch {
		t.Errorf("Open(\"?\") mode = %v, want ModeSearch", got)
	}
}

func TestPalette_DefaultSortByActivity(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())
	m.SetCounts(map[string]int64{"c1": 5}, map[string]int64{"c3": 2})
	m.Open("")

	view := testutil.StripANSI(m.View())
	// c3 has mentions (top), c1 has unread (second), c2 only recency (third).
	iRandom := strings.Index(view, "Random")
	iGeneral := strings.Index(view, "General")
	iIncidents := strings.Index(view, "Incidents")
	if iRandom == -1 || iGeneral == -1 || iIncidents == -1 {
		t.Fatalf("expected all channels in view:\n%s", view)
	}
	if iRandom >= iGeneral || iGeneral >= iIncidents {
		t.Errorf("expected order Random, General, Incidents; got view:\n%s", view)
	}
}

func TestPalette_RecencySortWithoutBadges(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())
	m.Open("")

	view := testutil.StripANSI(m.View())
	// LastPostAt: Incidents(300) > Random(200) > General(100).
	iIncidents := strings.Index(view, "Incidents")
	iRandom := strings.Index(view, "Random")
	iGeneral := strings.Index(view, "General")
	if iIncidents >= iRandom || iRandom >= iGeneral {
		t.Errorf("expected recency order Incidents, Random, General:\n%s", view)
	}
}

func TestPalette_UnreadBadgesRendered(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())
	m.SetCounts(map[string]int64{"c1": 5}, map[string]int64{"c1": 2})
	m.Open("")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "(5)") || !strings.Contains(view, "[@2]") {
		t.Errorf("expected unread and mention badges in view:\n%s", view)
	}
}

func TestPalette_FilterChannels(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())
	m.Open("")
	m, _ = typeRunes(m, "gen")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "General") {
		t.Errorf("expected General to match 'gen':\n%s", view)
	}
	if strings.Contains(view, "Random") {
		t.Errorf("expected Random filtered out by 'gen':\n%s", view)
	}
}

func TestPalette_MultiTeamSuffix(t *testing.T) {
	m := newPalette()
	m.SetTeams([]*model.Team{
		{ID: "t1", DisplayName: "Engineering"},
		{ID: "t2", DisplayName: "Design"},
	})
	m.SetChannels([]*model.Channel{{ID: "c1", DisplayName: "General", TeamID: "t1"}})
	m.Open("")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "General · Engineering") {
		t.Errorf("expected team suffix on channel row:\n%s", view)
	}
}

func TestPalette_SingleTeamNoSuffix(t *testing.T) {
	m := newPalette()
	m.SetTeams([]*model.Team{{ID: "t1", DisplayName: "Engineering"}})
	m.SetChannels([]*model.Channel{{ID: "c1", DisplayName: "General", TeamID: "t1"}})
	m.Open("")

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "· Engineering") {
		t.Errorf("expected no team suffix with a single team:\n%s", view)
	}
}

func TestPalette_EnterChoosesChannel(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())
	m.Open("")
	m, _ = typeRunes(m, "gen")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	chosen, ok := msg.(palette.ChannelChosenMsg)
	if !ok {
		t.Fatalf("expected ChannelChosenMsg, got %T", msg)
	}
	if chosen.Channel.ID != "c1" {
		t.Errorf("expected channel c1, got %s", chosen.Channel.ID)
	}
	if m.Visible() {
		t.Error("expected palette to close after choosing")
	}
}

func TestPalette_DMDisplayNamesUsed(t *testing.T) {
	m := newPalette()
	names := map[string]string{"dm1": "Bob Smith"}
	m.SetDMDisplayNames(names)
	m.SetDMChannels([]*model.Channel{{ID: "dm1", Name: "u1__u2", Type: model.ChannelDirect}})
	m.Open("")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Bob Smith") {
		t.Errorf("expected DM display name in rows:\n%s", view)
	}
}

func TestPalette_CommandMode(t *testing.T) {
	m := newPalette()
	m.SetCommands([]*model.Command{
		{ID: "cmd1", Slug: "remind", Description: "Set reminder"},
		{ID: "cmd2", Slug: "away", Description: "Set away"},
	})
	m.Open("/")
	m, _ = typeRunes(m, "rem")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "/remind") {
		t.Errorf("expected /remind in filtered commands:\n%s", view)
	}
	if strings.Contains(view, "/away") {
		t.Errorf("expected /away filtered out:\n%s", view)
	}

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	if chosen, ok := cmd().(palette.CommandChosenMsg); !ok || chosen.Command.Slug != "remind" {
		t.Errorf("expected CommandChosenMsg{remind}, got %v", cmd())
	}
	_ = m
}

func TestPalette_UserModeDebounceEmitsQuery(t *testing.T) {
	m := newPalette()
	m.Open("@")
	m, cmd := typeRunes(m, "bo")
	if cmd == nil {
		t.Fatal("expected debounce tick command after typing")
	}

	// Simulate the debounce timer firing with the latest generation.
	m, cmd = m.Update(palette.DebounceMsg{Gen: currentGen(t, m)})
	if cmd == nil {
		t.Fatal("expected UserQueryMsg command from debounce")
	}
	q, ok := cmd().(palette.UserQueryMsg)
	if !ok {
		t.Fatalf("expected UserQueryMsg, got %T", cmd())
	}
	if q.Term != "bo" {
		t.Errorf("expected term 'bo', got %q", q.Term)
	}
}

// currentGen extracts the live debounce generation by firing increasing
// generations until one produces a command.
func currentGen(t *testing.T, m palette.Model) int {
	t.Helper()
	for gen := 1; gen < 100; gen++ {
		if _, cmd := m.Update(palette.DebounceMsg{Gen: gen}); cmd != nil {
			return gen
		}
	}
	t.Fatal("no live debounce generation found")
	return 0
}

func TestPalette_StaleDebounceIgnored(t *testing.T) {
	m := newPalette()
	m.Open("@")
	m, _ = typeRunes(m, "bo")
	live := currentGen(t, m)

	if _, cmd := m.Update(palette.DebounceMsg{Gen: live - 1}); cmd != nil {
		t.Error("stale debounce generation should not emit a query")
	}
}

func TestPalette_UserModeEnterSelectsResult(t *testing.T) {
	m := newPalette()
	m.Open("@")
	m, _ = typeRunes(m, "bo")
	m.SetUsers([]*model.User{{ID: "u2", Username: "bob", DisplayName: "Bob Smith"}})

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "@bob (Bob Smith)") {
		t.Errorf("expected user row in view:\n%s", view)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	if chosen, ok := cmd().(palette.UserChosenMsg); !ok || chosen.User.Username != "bob" {
		t.Errorf("expected UserChosenMsg{bob}, got %v", cmd())
	}
}

func TestPalette_SearchModeSubmitAndSelect(t *testing.T) {
	m := newPalette()
	m.SetActiveChannel("General")
	m.Open("?")
	m, _ = typeRunes(m, "deploy")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Search in General") {
		t.Errorf("expected search scope header:\n%s", view)
	}

	// Enter with no results submits the search.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	sub, ok := cmd().(palette.SearchSubmitMsg)
	if !ok || sub.Term != "deploy" {
		t.Fatalf("expected SearchSubmitMsg{deploy}, got %v", cmd())
	}

	// Results arrive; Enter now selects the highlighted post.
	m.SetUsernames(map[string]string{"u2": "bob"})
	m.SetSearchResults([]*model.Post{{ID: "p1", UserID: "u2", Content: "deploy failed"}})
	view = testutil.StripANSI(m.View())
	if !strings.Contains(view, "bob: deploy failed") {
		t.Errorf("expected search result row:\n%s", view)
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter on result")
	}
	if chosen, ok := cmd().(palette.PostChosenMsg); !ok || chosen.Post.ID != "p1" {
		t.Errorf("expected PostChosenMsg{p1}, got %v", cmd())
	}
}

func TestPalette_SearchModeNoActiveChannel(t *testing.T) {
	m := newPalette()
	m.Open("?")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "No active channel") {
		t.Errorf("expected no-active-channel notice:\n%s", view)
	}
}

func TestPalette_FooterHints(t *testing.T) {
	m := newPalette()
	m.Open("")

	view := testutil.StripANSI(m.View())
	for _, hint := range []string{"@ people", "/ commands", "? search", "esc close"} {
		if !strings.Contains(view, hint) {
			t.Errorf("expected footer hint %q:\n%s", hint, view)
		}
	}
}

func TestPalette_EscCloses(t *testing.T) {
	m := newPalette()
	m.Open("")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Visible() {
		t.Error("expected Esc to close the palette")
	}
}

func TestPalette_RowAtMapsClicks(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())
	m.Open("")

	// Overlay layout: chrome (2) + input (1) + context (1), rows follow.
	if _, ok := m.RowAt(3); ok {
		t.Error("expected no row on the context line")
	}
	idx, ok := m.RowAt(4)
	if !ok || idx != 0 {
		t.Errorf("expected first row at localY=4, got idx=%d ok=%v", idx, ok)
	}
	idx, ok = m.RowAt(6)
	if !ok || idx != 2 {
		t.Errorf("expected third row at localY=6, got idx=%d ok=%v", idx, ok)
	}
	if _, ok := m.RowAt(7); ok {
		t.Error("expected no row past the result list")
	}
}

func TestPalette_ChooseRowEmitsChannel(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())
	m.Open("")

	cmd := m.ChooseRow(0)
	if cmd == nil {
		t.Fatal("expected command from ChooseRow")
	}
	if _, ok := cmd().(palette.ChannelChosenMsg); !ok {
		t.Errorf("expected ChannelChosenMsg, got %T", cmd())
	}
	if m.Visible() {
		t.Error("expected palette to close after ChooseRow")
	}
}

func TestPalette_MoveCursorClamped(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())
	m.Open("")

	m.MoveCursor(-5)
	if _, ok := m.RowAt(4); !ok {
		t.Error("cursor clamp should keep window at top")
	}
	m.MoveCursor(100)
	m.MoveCursor(-1)
	// No panic and still selectable.
	if cmd := m.ChooseRow(1); cmd == nil {
		t.Error("expected ChooseRow to work after cursor movement")
	}
}

func TestPalette_FocusBlurAndStyles(t *testing.T) {
	m := newPalette()
	m.Open("")
	if !m.Focused() {
		t.Error("expected focused after Open")
	}
	m.Blur()
	if m.Focused() {
		t.Error("expected blurred after Blur")
	}
	m.Focus()
	if !m.Focused() {
		t.Error("expected focused after Focus")
	}
	m.SetStyles(styles.New(theme.TokyoNight()))
	if m.View() == "" {
		t.Error("expected non-empty view after SetStyles")
	}
}

func TestPalette_TeamNameFallsBackToSlug(t *testing.T) {
	m := newPalette()
	m.SetTeams([]*model.Team{
		{ID: "t1", Name: "eng"},
		{ID: "t2", DisplayName: "Design"},
	})
	m.SetChannels([]*model.Channel{{ID: "c1", DisplayName: "General", TeamID: "t1"}})
	m.Open("")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "General · eng") {
		t.Errorf("expected team slug fallback in suffix:\n%s", view)
	}
}

func TestPalette_ChannelNameFallsBackToSlug(t *testing.T) {
	m := newPalette()
	m.SetChannels([]*model.Channel{{ID: "c1", Name: "town-square"}})
	m.Open("")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "town-square") {
		t.Errorf("expected channel Name fallback:\n%s", view)
	}
}

func TestPalette_SearchResultUnknownAuthorAndLongContent(t *testing.T) {
	m := newPalette()
	m.SetActiveChannel("General")
	m.Open("?")
	m, _ = typeRunes(m, "x")
	m.SetSearchResults([]*model.Post{{
		ID:      "p1",
		UserID:  "user-abcdef123456",
		Content: strings.Repeat("long content ", 20),
	}})

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "user-abc") {
		t.Errorf("expected truncated user ID as author:\n%s", view)
	}
	if !strings.Contains(view, "…") {
		t.Errorf("expected truncated excerpt:\n%s", view)
	}
}

func TestPalette_CursorScrollsWindow(t *testing.T) {
	m := palette.New(styles.New(theme.TokyoNight()))
	m.SetSize(120, 16) // maxVisibleRows = max(16/2-6, 5) = 5
	channels := make([]*model.Channel, 10)
	for i := range channels {
		channels[i] = &model.Channel{
			ID:          fmt.Sprintf("c%d", i),
			DisplayName: fmt.Sprintf("chan-%02d", i),
			LastPostAt:  int64(100 - i),
		}
	}
	m.SetChannels(channels)
	m.Open("")

	// Move the cursor past the visible window; the window follows.
	for range 8 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "chan-08") {
		t.Errorf("expected window to scroll to cursor:\n%s", view)
	}
	if strings.Contains(view, "chan-00") {
		t.Errorf("expected first row scrolled out of window:\n%s", view)
	}

	// RowAt maps the first visible line to the window start, not row 0.
	idx, ok := m.RowAt(4)
	if !ok || idx != 4 {
		t.Errorf("expected RowAt(4) = window start 4, got idx=%d ok=%v", idx, ok)
	}
}

func TestPalette_EnterOnEmptySearchIsNoOp(t *testing.T) {
	m := newPalette()
	m.SetActiveChannel("General")
	m.Open("?")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command for empty search submit")
	}
}

func TestPalette_EnterOnEmptyUserQueryIsNoOp(t *testing.T) {
	m := newPalette()
	m.Open("@")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command for empty people query")
	}
}

func TestPalette_UpdateIgnoredWhenHidden(t *testing.T) {
	m := newPalette()
	m.SetChannels(testChannels())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command while hidden")
	}
}
