package tui_test

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

// Opening a thread and going back used to re-render the whole loaded
// history through glamour, twice over, though nothing about it had changed.
func BenchmarkModel_OpenAndCloseThread(b *testing.B) {
	m := testModel()
	order := make([]*model.Post, 300)
	for i := range order {
		order[len(order)-1-i] = &model.Post{
			ID: fmt.Sprintf("p%d", i), ChannelID: "c1", UserID: "u1",
			Content:  fmt.Sprintf("message %d with **bold** and `code`", i),
			CreateAt: int64(1700000000000 + i*60000),
		}
	}
	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 120, Height: 40},
		tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}},
		tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}},
		tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}}},
		tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: order}},
	} {
		updated, _ := m.Update(msg)
		m = updated.(tui.Model)
	}
	root := order[0]
	esc := tea.KeyMsg{Type: tea.KeyEscape}

	b.ResetTimer()
	for range b.N {
		updated, _ := m.Update(viewport.PostSelectedMsg{Post: root})
		updated, _ = updated.(tui.Model).Update(esc)
		m = updated.(tui.Model)
	}
}
