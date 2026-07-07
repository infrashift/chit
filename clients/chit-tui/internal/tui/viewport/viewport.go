package viewport

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	bvp "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/post"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// PostSelectedMsg is sent when a post is selected (for thread view).
type PostSelectedMsg struct {
	Post *model.Post
}

// Model is the viewport component that displays posts.
type Model struct {
	viewport        bvp.Model
	posts           []*model.Post
	usernames       map[string]string
	currentUsername string
	threadCounts    map[string]int
	postTags        map[string][]string // postID -> tag names
	cursor          int
	postLineOffsets []int // line offset of each post in the rendered content
	focused         bool
	styles          styles.Styles
	width           int
	height          int
	renderer        *glamour.TermRenderer
	cache           map[string]string // postID -> rendered string
}

// New creates a new viewport model.
func New(s styles.Styles) Model {
	return Model{
		viewport:     bvp.New(0, 0),
		usernames:    make(map[string]string),
		threadCounts: make(map[string]int),
		postTags:     make(map[string][]string),
		styles:       s,
		cache:        make(map[string]string),
	}
}

// SetPosts replaces the current posts, reversing the order so that the oldest
// post appears at the top and the newest at the bottom.
func (m *Model) SetPosts(posts []*model.Post) {
	reversed := make([]*model.Post, len(posts))
	for i, p := range posts {
		reversed[len(posts)-1-i] = p
	}
	m.posts = reversed
	m.cursor = len(reversed) - 1
	m.cache = make(map[string]string)
	m.updateContent()
	m.viewport.GotoBottom()
}

// AppendPost adds a post at the end.
func (m *Model) AppendPost(p *model.Post) {
	m.posts = append(m.posts, p)
	m.updateContent()
	m.viewport.GotoBottom()
}

// SetUsernames updates the username map and re-renders only posts whose
// displayed username actually changed.
func (m *Model) SetUsernames(names map[string]string) {
	// Invalidate cache entries where the username changed.
	for _, p := range m.posts {
		oldName := m.usernames[p.UserID]
		newName := names[p.UserID]
		if oldName != newName {
			delete(m.cache, p.ID)
		}
	}
	m.usernames = names
	m.updateContent()
}

// SetCurrentUsername sets the current user's username for mention highlighting.
func (m *Model) SetCurrentUsername(username string) {
	m.currentUsername = username
	m.cache = make(map[string]string)
	m.updateContent()
}

// SetThreadCounts updates the thread reply counts and re-renders affected posts.
func (m *Model) SetThreadCounts(counts map[string]int) {
	for _, p := range m.posts {
		oldCount := m.threadCounts[p.ID]
		newCount := counts[p.ID]
		if oldCount != newCount {
			delete(m.cache, p.ID)
		}
	}
	// Defensive copy to avoid aliasing with the caller's map.
	m.threadCounts = make(map[string]int, len(counts))
	for k, v := range counts {
		m.threadCounts[k] = v
	}
	m.updateContent()
}

// SetPostTags updates the tag names for a post and invalidates its cache.
func (m *Model) SetPostTags(postID string, tagNames []string) {
	m.postTags[postID] = tagNames
	delete(m.cache, postID)
	m.updateContent()
}

func (m Model) postTagNames(postID string) []string {
	return m.postTags[postID]
}

// Posts returns the current posts.
func (m Model) Posts() []*model.Post { return m.posts }

// SelectedPost returns the post under the cursor.
func (m Model) SelectedPost() *model.Post {
	if m.cursor >= 0 && m.cursor < len(m.posts) {
		return m.posts[m.cursor]
	}
	return nil
}

// SetStyles replaces the styles and re-renders all posts.
// Resets the renderer so it picks up the new theme's markdown colors.
func (m *Model) SetStyles(s styles.Styles) {
	m.styles = s
	m.renderer = nil
	m.cache = make(map[string]string)
	m.updateContent()
}

// Focus sets focus state and re-renders to show the selection highlight.
func (m *Model) Focus() {
	m.focused = true
	m.updateContent()
}

// Blur removes focus and re-renders to remove the selection highlight.
func (m *Model) Blur() {
	m.focused = false
	m.updateContent()
}

// Focused returns the focus state.
func (m Model) Focused() bool { return m.focused }

// SetSize sets the viewport dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w - 2
	m.viewport.Height = h - 2
	// Width changed — need a new renderer and full re-render.
	m.renderer = nil
	m.cache = make(map[string]string)
	m.updateContent()
}

// ScrollBy scrolls the content by the given number of lines (negative = up).
// It works regardless of focus so the mouse wheel can scroll an unfocused pane.
func (m *Model) ScrollBy(lines int) {
	m.viewport.SetYOffset(m.viewport.YOffset + lines)
}

// YOffset returns the current scroll offset in lines.
func (m Model) YOffset() int { return m.viewport.YOffset }

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.cursor > 0 {
			m.cursor--
			m.updateContent()
			m.scrollToCursor()
		}
		return m, nil
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("j", "down"))):
		if m.cursor < len(m.posts)-1 {
			m.cursor++
			m.updateContent()
			m.scrollToCursor()
		}
		return m, nil
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter"))):
		p := m.SelectedPost()
		if p != nil {
			return m, func() tea.Msg { return PostSelectedMsg{Post: p} }
		}
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the viewport.
func (m Model) View() string {
	borderStyle := m.styles.Viewport
	if m.focused {
		borderStyle = borderStyle.BorderForeground(m.styles.ActiveBorder.GetBorderBottomForeground())
	}
	return borderStyle.Width(m.width - 2).Height(m.height - 2).Render(m.viewport.View())
}

func (m *Model) ensureRenderer() {
	if m.renderer == nil {
		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithStyles(m.styles.MarkdownStyleConfig),
			glamour.WithWordWrap(max(m.width-8, 20)),
			glamour.WithPreservedNewLines(),
			glamour.WithEmoji(),
		)
	}
}

// sameDay returns true if both times fall on the same calendar date.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// formatDaySeparator produces a centered day label like ───── Monday 2026-03-10 ─────.
func formatDaySeparator(t time.Time, width int) string {
	label := fmt.Sprintf(" %s %s ", t.Format("Monday"), t.Format("2006-01-02"))
	labelWidth := len([]rune(label))
	remaining := width - labelWidth
	if remaining < 2 {
		return label
	}
	left := remaining / 2
	right := remaining - left
	return strings.Repeat("─", left) + label + strings.Repeat("─", right)
}

func (m *Model) updateContent() {
	m.ensureRenderer()
	var lines []string
	m.postLineOffsets = make([]int, len(m.posts))
	lineCount := 0
	var prevDate time.Time
	for i, p := range m.posts {
		curDate := model.MillisToTime(p.CreateAt)
		if i > 0 && !sameDay(prevDate, curDate) {
			sep := m.styles.DaySeparator.Render(formatDaySeparator(curDate, m.width-4))
			lines = append(lines, sep)
			lineCount += strings.Count(sep, "\n") + 1 + 1 // +1 for blank line separator
		}
		prevDate = curDate

		m.postLineOffsets[i] = lineCount
		if cached, ok := m.cache[p.ID]; ok {
			rendered := cached
			if i == m.cursor && m.focused {
				rendered = m.styles.SelectedPost.Render(rendered)
			}
			lines = append(lines, rendered)
			lineCount += strings.Count(rendered, "\n") + 1 + 1 // +1 for the blank line separator
			continue
		}
		username := m.usernames[p.UserID]
		if username == "" {
			username = p.UserID[:min(8, len(p.UserID))]
		}
		pb := post.New(p, username, m.styles, m.width-4, m.renderer, m.currentUsername, m.threadCounts[p.ID], m.postTagNames(p.ID))
		rendered := pb.View()
		m.cache[p.ID] = rendered
		display := rendered
		if i == m.cursor && m.focused {
			display = m.styles.SelectedPost.Render(rendered)
		}
		lines = append(lines, display)
		lineCount += strings.Count(display, "\n") + 1 + 1
	}
	m.viewport.SetContent(strings.Join(lines, "\n\n"))
}

// scrollToCursor adjusts the viewport offset so the cursor post is visible.
func (m *Model) scrollToCursor() {
	if m.cursor < 0 || m.cursor >= len(m.postLineOffsets) {
		return
	}
	target := m.postLineOffsets[m.cursor]
	if target < m.viewport.YOffset {
		m.viewport.SetYOffset(target)
	} else if target >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(target - m.viewport.Height + 3)
	}
}
