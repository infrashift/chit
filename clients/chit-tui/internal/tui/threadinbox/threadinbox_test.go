package threadinbox_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/threadinbox"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func newInbox() threadinbox.Model {
	m := threadinbox.New(styles.New(theme.TokyoNight()))
	m.SetSize(120, 40)
	m.SetChannelNames(map[string]string{"c1": "General", "c2": "Design", "d1": "bob", "g1": "bob, carol"})
	m.SetTeamName("Eng")
	m.Open()
	return m
}

func thread(rootID, channelID, content string, replies int, lastReply, lastViewed int64) *model.ThreadResponse {
	return &model.ThreadResponse{
		Thread: &model.Thread{
			PostID:      rootID,
			ChannelID:   channelID,
			ReplyCount:  replies,
			LastReplyAt: lastReply,
		},
		Posts:        []*model.Post{{ID: rootID, ChannelID: channelID, Content: content}},
		LastViewedAt: lastViewed,
	}
}

func key(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	if s == "esc" {
		return tea.KeyMsg{Type: tea.KeyEscape}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// An empty box while the list is still in flight reads as "you follow no
// threads", which is a different and discouraging answer.
func TestInbox_ShowsLoadingBeforeTheListArrives(t *testing.T) {
	m := newInbox()

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Loading") {
		t.Errorf("no loading state:\n%s", view)
	}
	if strings.Contains(view, "not following any threads") {
		t.Errorf("claimed there are no threads before the list arrived:\n%s", view)
	}
}

func TestInbox_ShowsAnEmptyStateWithAWayForward(t *testing.T) {
	m := newInbox()
	m.SetThreads(nil, nil)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "not following any threads") {
		t.Errorf("no empty state:\n%s", view)
	}
	if !strings.Contains(view, "Reply to a message") {
		t.Errorf("empty state does not say how to follow one:\n%s", view)
	}
}

// A row has to say which channel it is in and what it is about. Without the
// root message, several threads in one channel are indistinguishable.
func TestInbox_RowShowsChannelAndSummary(t *testing.T) {
	m := newInbox()
	m.SetThreads([]*model.ThreadResponse{
		thread("p1", "c1", "the deploy is stuck again", 3, 200, 200),
	}, nil)

	view := testutil.StripANSI(m.View())
	for _, want := range []string{"#General", "the deploy is stuck again", "3 replies"} {
		if !strings.Contains(view, want) {
			t.Errorf("row is missing %q:\n%s", want, view)
		}
	}
}

// The unread marker is the reason to open an inbox at all.
func TestInbox_MarksUnreadThreads(t *testing.T) {
	m := newInbox()
	m.SetThreads([]*model.ThreadResponse{
		thread("p1", "c1", "unread one", 1, 300, 200), // replied after last read
		thread("p2", "c2", "read one", 1, 100, 200),   // read since the last reply
	}, nil)

	lines := strings.Split(testutil.StripANSI(m.View()), "\n")
	var unreadLine, readLine string
	for _, l := range lines {
		if strings.Contains(l, "unread one") {
			unreadLine = l
		}
		if strings.Contains(l, "read one") && !strings.Contains(l, "unread one") {
			readLine = l
		}
	}
	if !strings.Contains(unreadLine, "●") {
		t.Errorf("thread with replies since the last read is not marked unread: %q", unreadLine)
	}
	if strings.Contains(readLine, "●") {
		t.Errorf("already-read thread is marked unread: %q", readLine)
	}
}

// A thread never read has last_viewed_at of zero, which must count as unread
// rather than as "viewed at the epoch".
func TestInbox_NeverReadCountsAsUnread(t *testing.T) {
	m := newInbox()
	m.SetThreads([]*model.ThreadResponse{thread("p1", "c1", "brand new", 1, 100, 0)}, nil)

	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "●") {
		t.Errorf("a never-read thread is not marked unread:\n%s", view)
	}
}

func TestInbox_EnterOpensTheSelectedThread(t *testing.T) {
	m := newInbox()
	m.SetThreads([]*model.ThreadResponse{
		thread("p1", "c1", "first", 1, 100, 0),
		thread("p2", "c2", "second", 1, 100, 0),
	}, nil)

	m, _ = m.Update(key("j")) // move to the second
	m, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter produced no message")
	}
	chosen, ok := cmd().(threadinbox.ThreadChosenMsg)
	if !ok {
		t.Fatalf("expected a chosen thread, got %T", cmd())
	}
	if chosen.RootID != "p2" || chosen.ChannelID != "c2" {
		t.Errorf("opened %+v, want p2 in c2", chosen)
	}
	if m.Visible() {
		t.Error("the overlay stayed open after opening a thread")
	}
}

// Unfollowing removes the thread from the list it defines: leaving the row
// behind would show something the next fetch will not return.
func TestInbox_UnfollowRemovesTheRow(t *testing.T) {
	m := newInbox()
	m.SetThreads([]*model.ThreadResponse{
		thread("p1", "c1", "first", 1, 100, 0),
		thread("p2", "c2", "second", 1, 100, 0),
	}, nil)

	m, cmd := m.Update(key("u"))
	if cmd == nil {
		t.Fatal("u produced no message")
	}
	msg, ok := cmd().(threadinbox.FollowToggledMsg)
	if !ok {
		t.Fatalf("expected a follow change, got %T", cmd())
	}
	if msg.RootID != "p1" || msg.Following {
		t.Errorf("unfollowed %+v, want p1 with following=false", msg)
	}
	if len(m.Threads()) != 1 || m.Threads()[0].Thread.PostID != "p2" {
		t.Errorf("list still holds the unfollowed thread: %+v", m.Threads())
	}
	if view := testutil.StripANSI(m.View()); strings.Contains(view, "first") {
		t.Errorf("unfollowed thread is still rendered:\n%s", view)
	}
}

// Unfollowing the last row must not leave the cursor past the end.
func TestInbox_UnfollowingTheLastRowKeepsTheCursorInRange(t *testing.T) {
	m := newInbox()
	m.SetThreads([]*model.ThreadResponse{thread("p1", "c1", "only", 1, 100, 0)}, nil)

	m, _ = m.Update(key("u"))
	// Any further key must not panic on an out-of-range cursor.
	m, _ = m.Update(key("u"))
	m, _ = m.Update(key("enter"))

	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "not following any threads") {
		t.Errorf("expected the empty state:\n%s", view)
	}
}

func TestInbox_EscClosesAndReports(t *testing.T) {
	m := newInbox()
	m.SetThreads([]*model.ThreadResponse{thread("p1", "c1", "x", 0, 1, 1)}, nil)

	m, cmd := m.Update(key("esc"))
	if cmd == nil {
		t.Fatal("esc produced no message")
	}
	if _, ok := cmd().(threadinbox.ClosedMsg); !ok {
		t.Fatalf("expected ClosedMsg, got %T", cmd())
	}
	if m.Visible() {
		t.Error("the overlay stayed visible after esc")
	}
}

// A thread whose root the server did not send must still render — the row is
// what identifies it, and a nil root should not blank the list or panic.
func TestInbox_SurvivesAMissingRootPost(t *testing.T) {
	m := newInbox()
	m.SetThreads([]*model.ThreadResponse{{
		Thread: &model.Thread{PostID: "p1", ChannelID: "c1", LastReplyAt: 100},
	}}, nil)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "#General") {
		t.Errorf("row vanished without a root post:\n%s", view)
	}
	if !strings.Contains(view, "(no message)") {
		t.Errorf("no placeholder for the missing root:\n%s", view)
	}
}

// Threads in direct and group channels belong to no team, so they come from
// their own list and are shown in their own section, labeled by who the
// conversation is with rather than as a #channel.
func TestInbox_ListsDirectThreadsInTheirOwnSection(t *testing.T) {
	m := newInbox()
	m.SetThreads(
		[]*model.ThreadResponse{thread("p1", "c1", "team thread", 1, 100, 100)},
		[]*model.ThreadResponse{thread("p2", "d1", "dm thread", 1, 100, 100)},
	)

	view := testutil.StripANSI(m.View())
	for _, want := range []string{"In Eng", "#General", "team thread", "Direct messages", "bob", "dm thread"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "#bob") {
		t.Errorf("a DM is labeled as a #channel:\n%s", view)
	}
	if strings.Index(view, "team thread") > strings.Index(view, "Direct messages") {
		t.Errorf("the team section should come first:\n%s", view)
	}
}

func TestInbox_OnlyDirectThreads(t *testing.T) {
	m := newInbox()
	m.SetThreads(nil, []*model.ThreadResponse{thread("p2", "g1", "group thread", 1, 100, 100)})

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "In Eng") {
		t.Errorf("an empty team section has a heading:\n%s", view)
	}
	if !strings.Contains(view, "Direct messages") || !strings.Contains(view, "bob, carol") {
		t.Errorf("the group thread is not listed:\n%s", view)
	}
}

// The cursor moves across both sections as one list.
func TestInbox_EnterOpensADirectThread(t *testing.T) {
	m := newInbox()
	m.SetThreads(
		[]*model.ThreadResponse{thread("p1", "c1", "team thread", 1, 100, 100)},
		[]*model.ThreadResponse{thread("p2", "d1", "dm thread", 1, 100, 100)},
	)

	m, _ = m.Update(key("j"))
	_, cmd := m.Update(key("enter"))
	chosen, ok := cmd().(threadinbox.ThreadChosenMsg)
	if !ok || chosen.RootID != "p2" || chosen.ChannelID != "d1" {
		t.Fatalf("opened %+v, want p2 in d1", cmd())
	}
}

// Unfollowing in the team section must not shift a DM thread into it.
func TestInbox_UnfollowKeepsTheSections(t *testing.T) {
	m := newInbox()
	m.SetThreads(
		[]*model.ThreadResponse{thread("p1", "c1", "team thread", 1, 100, 100)},
		[]*model.ThreadResponse{thread("p2", "d1", "dm thread", 1, 100, 100)},
	)

	m, _ = m.Update(key("u"))
	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "In Eng") || strings.Contains(view, "#bob") {
		t.Errorf("the DM thread moved into the team section:\n%s", view)
	}
	if !strings.Contains(view, "Direct messages") || !strings.Contains(view, "dm thread") {
		t.Errorf("the DM thread is gone:\n%s", view)
	}

	// And unfollowing the last DM thread empties the inbox cleanly.
	m, cmd := m.Update(key("u"))
	if msg, ok := cmd().(threadinbox.FollowToggledMsg); !ok || msg.RootID != "p2" {
		t.Fatalf("unfollowed %+v, want p2", cmd())
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "not following any threads") {
		t.Errorf("expected the empty state:\n%s", view)
	}
}

// Without a team name the heading still says what the section is; a DM whose
// name is not known yet is a bare "?", not a "#?" channel.
func TestInbox_FallbackLabels(t *testing.T) {
	m := threadinbox.New(styles.New(theme.TokyoNight()))
	m.SetSize(120, 40)
	m.Open()
	m.SetThreads(
		[]*model.ThreadResponse{thread("p1", "c9", "team thread", 1, 100, 100)},
		[]*model.ThreadResponse{thread("p2", "d9", "dm thread", 1, 100, 100)},
	)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "In this team") {
		t.Errorf("no fallback team heading:\n%s", view)
	}
	if !strings.Contains(view, "#?  team thread") || !strings.Contains(view, " ?  dm thread") {
		t.Errorf("unexpected fallback labels:\n%s", view)
	}
}
