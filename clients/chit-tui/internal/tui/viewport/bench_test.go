package viewport_test

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

// history returns n posts newest-first, the order the API returns them in,
// with enough markdown that rendering them is representative.
func history(n int) []*model.Post {
	posts := make([]*model.Post, n)
	for i := range n {
		posts[n-1-i] = &model.Post{
			ID: fmt.Sprintf("p%d", i), UserID: "u1",
			Content:  fmt.Sprintf("message %d with **bold**, `code` and a [link](https://example.com)", i),
			CreateAt: int64(1700000000000 + i*60000),
		}
	}
	return posts
}

func loaded(b *testing.B, n int) viewport.Model {
	b.Helper()
	m := viewport.New(testStyles())
	m.SetSize(120, 40)
	m.SetPosts(history(n))
	m.Focus()
	return m
}

// Moving the cursor is the most frequent thing done in the history pane.
func BenchmarkViewport_MoveCursor(b *testing.B) {
	m := loaded(b, 500)
	up := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	b.ResetTimer()
	for i := range b.N {
		if i%2 == 0 {
			m, _ = m.Update(up)
		} else {
			m, _ = m.Update(down)
		}
	}
}

// The root model re-applies the layout on every channel switch and thread
// open or close, usually without the size having changed.
func BenchmarkViewport_SetSizeUnchanged(b *testing.B) {
	m := loaded(b, 500)
	b.ResetTimer()
	for range b.N {
		m.SetSize(120, 40)
	}
}

// Tags arrive in one batch for a whole page of history.
func BenchmarkViewport_TagAPage(b *testing.B) {
	m := loaded(b, 60)
	tags := make(map[string][]string, 60)
	for _, p := range m.Posts() {
		tags[p.ID] = []string{"urgent"}
	}
	b.ResetTimer()
	for range b.N {
		m.SetPostsTags(tags)
	}
}

func BenchmarkViewport_FocusToggle(b *testing.B) {
	m := loaded(b, 500)
	b.ResetTimer()
	for range b.N {
		m.Blur()
		m.Focus()
	}
}
