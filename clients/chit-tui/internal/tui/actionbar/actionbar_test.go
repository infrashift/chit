package actionbar_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/actionbar"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func newBar(width int) actionbar.Model {
	m := actionbar.New(styles.New(theme.TokyoNight()))
	m.SetSize(width)
	return m
}

func TestView_ContainsButtonsAndContext(t *testing.T) {
	m := newBar(120)
	m.SetContext("Engineering", "General", "alice")

	view := testutil.StripANSI(m.View())
	for _, want := range []string{"[^K Jump]", "[^S Search]", "[^D DM]", "[^N New]", "Engineering > General", "alice"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected %q in bar view:\n%s", want, view)
		}
	}
}

func TestView_OmitsChannelSeparatorWhenNoChannel(t *testing.T) {
	m := newBar(120)
	m.SetContext("Engineering", "", "alice")

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, ">") {
		t.Errorf("expected no channel separator without a channel:\n%s", view)
	}
}

func TestView_FillsWidth(t *testing.T) {
	m := newBar(120)
	m.SetContext("Engineering", "General", "alice")

	if got := ansi.StringWidth(m.View()); got != 120 {
		t.Errorf("expected bar width 120, got %d", got)
	}
}

func TestView_ConnectionIndicator(t *testing.T) {
	m := newBar(120)

	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "○") {
		t.Errorf("expected disconnected indicator ○:\n%s", view)
	}
	m.SetConnected(true)
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "●") {
		t.Errorf("expected connected indicator ●:\n%s", view)
	}
}

func TestView_ShowsError(t *testing.T) {
	m := newBar(120)
	m.SetError("boom failed")

	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "boom failed") {
		t.Errorf("expected error text in bar:\n%s", view)
	}
}

func TestHitTest_ButtonBoundaries(t *testing.T) {
	m := newBar(120)

	// Bar layout: one leading space, then "[^K Jump]" (9 cells) at [1,10),
	// a space, then "[^S Search]" (11 cells) at [11,22).
	cases := []struct {
		x    int
		want actionbar.Action
	}{
		{0, actionbar.ActionNone},
		{1, actionbar.ActionPalette},
		{9, actionbar.ActionPalette},
		{10, actionbar.ActionNone},
		{11, actionbar.ActionSearch},
		{21, actionbar.ActionSearch},
		{119, actionbar.ActionNone},
	}
	for _, c := range cases {
		if got := m.HitTest(c.x); got != c.want {
			t.Errorf("HitTest(%d) = %v, want %v", c.x, got, c.want)
		}
	}
}

func TestHitTest_AllButtonsReachable(t *testing.T) {
	m := newBar(200)

	found := map[actionbar.Action]bool{}
	for x := range 200 {
		found[m.HitTest(x)] = true
	}
	for _, want := range []actionbar.Action{
		actionbar.ActionPalette, actionbar.ActionSearch,
		actionbar.ActionPeople, actionbar.ActionNewChannel,
	} {
		if !found[want] {
			t.Errorf("action %v not reachable via HitTest", want)
		}
	}
	if found[actionbar.ActionCloseThread] {
		t.Error("close-thread button should not be present when thread is closed")
	}
}

func TestHitTest_ThreadOpenAddsBackButton(t *testing.T) {
	m := newBar(200)
	m.SetThreadOpen(true)

	// "[esc Back]" (10 cells) leads the bar at [1,11).
	if got := m.HitTest(2); got != actionbar.ActionCloseThread {
		t.Errorf("HitTest(2) with thread open = %v, want ActionCloseThread", got)
	}
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "[esc Back]") {
		t.Errorf("expected back button in view:\n%s", view)
	}
}
