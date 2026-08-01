package viewport

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	bvp "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/ansi"
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
	postLineOffsets []int    // line offset of each post in the rendered content
	plainLines      []string // rendered content, one entry per line, ANSI stripped
	lineToPost      []int    // content line -> index into posts, -1 for separators
	selAnchor       int
	selHead         int
	selActive       bool
	searchTerm      string
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

// AppendPost adds a post at the end. A channel that started empty has no
// selection yet (cursor -1); select the first post that arrives so
// Enter/`t` work without reloading the channel.
func (m *Model) AppendPost(p *model.Post) {
	m.posts = append(m.posts, p)
	if m.cursor < 0 {
		m.cursor = len(m.posts) - 1
	}
	m.updateContent()
	m.viewport.GotoBottom()
}

// HasPost reports whether a post is already displayed. Both the HTTP response
// to sending and the WebSocket echo carry the same post, so whichever arrives
// second must not duplicate it.
func (m Model) HasPost(id string) bool {
	for _, p := range m.posts {
		if p.ID == id {
			return true
		}
	}
	return false
}

// UpdatePost replaces a post in place, keeping its position in the history,
// and reports whether it was present.
func (m *Model) UpdatePost(p *model.Post) bool {
	for i, existing := range m.posts {
		if existing.ID != p.ID {
			continue
		}
		m.posts[i] = p
		delete(m.cache, p.ID)
		m.updateContent()
		return true
	}
	return false
}

// RemovePost drops a post from the history and reports whether it was present.
func (m *Model) RemovePost(id string) bool {
	for i, p := range m.posts {
		if p.ID != id {
			continue
		}
		m.posts = append(m.posts[:i], m.posts[i+1:]...)
		delete(m.cache, id)
		if m.cursor >= len(m.posts) {
			m.cursor = len(m.posts) - 1
		}
		m.updateContent()
		return true
	}
	return false
}

// PrependPosts inserts older posts before the ones already loaded, keeping
// the reader where they are. Appending would put them at the wrong end, and
// re-setting the whole list would jump the view to the bottom.
func (m *Model) PrependPosts(older []*model.Post) {
	if len(older) == 0 {
		return
	}

	// Older arrives newest-first, like the rest of the history API; reverse
	// it so the combined list stays chronological.
	reversed := make([]*model.Post, len(older))
	for i, p := range older {
		reversed[len(older)-1-i] = p
	}

	linesBefore := len(m.plainLines)
	m.posts = append(reversed, m.posts...)
	m.cursor += len(reversed)
	m.updateContent()

	// Hold the reader's position: everything they were looking at has moved
	// down by however many lines were inserted above it.
	m.viewport.SetYOffset(m.viewport.YOffset + (len(m.plainLines) - linesBefore))
}

// AtTop reports whether the view is scrolled to the oldest loaded post, which
// is when there is any point fetching more.
func (m Model) AtTop() bool { return m.viewport.YOffset <= 0 }

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

// Bindings owned by this pane. They are exported so the root model's help
// overlay describes the same keys the pane actually handles, rather than a
// hand-maintained copy that can drift.
var (
	// ReplyKey opens the selected post as a thread, which is how a reply is
	// written.
	ReplyKey = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "reply in thread"),
	)
	// PrevPostKey and NextPostKey move the post cursor.
	PrevPostKey = key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "previous post"),
	)
	NextPostKey = key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "next post"),
	)
)

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
	case key.Matches(keyMsg, PrevPostKey):
		if m.cursor > 0 {
			m.cursor--
			m.updateContent()
			m.scrollToCursor()
		}
		return m, nil
	case key.Matches(keyMsg, NextPostKey):
		if m.cursor < len(m.posts)-1 {
			m.cursor++
			m.updateContent()
			m.scrollToCursor()
		}
		return m, nil
	case key.Matches(keyMsg, ReplyKey):
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
			rendered := m.highlightMatches(cached)
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
		display := m.highlightMatches(rendered)
		if i == m.cursor && m.focused {
			display = m.styles.SelectedPost.Render(rendered)
		}
		lines = append(lines, display)
		lineCount += strings.Count(display, "\n") + 1 + 1
	}
	content := strings.Join(lines, "\n\n")

	// Build the line map and the plain-text mirror the selection works from.
	// This is done on the joined content rather than per block so the indices
	// match what the viewport actually scrolls over, including the blank
	// separator lines.
	m.indexLines(content)

	if m.selActive {
		content = m.applySelection(content)
	}
	m.viewport.SetContent(content)
}

// indexLines records, for every line of rendered content, the post it belongs
// to and its text with styling removed. The reverse map is what turns a mouse
// row into a post, which post→line offsets alone cannot do.
func (m *Model) indexLines(content string) {
	raw := strings.Split(content, "\n")

	m.plainLines = make([]string, len(raw))
	m.lineToPost = make([]int, len(raw))
	for i, line := range raw {
		m.plainLines[i] = ansi.Strip(line)
		m.lineToPost[i] = -1
	}

	// postLineOffsets holds the first line of each post; everything up to the
	// next post's offset belongs to it.
	for i, start := range m.postLineOffsets {
		end := len(raw)
		if i+1 < len(m.postLineOffsets) {
			end = m.postLineOffsets[i+1]
		}
		for line := start; line < end && line < len(m.lineToPost); line++ {
			if line >= 0 {
				m.lineToPost[line] = i
			}
		}
	}
}

// applySelection re-renders the selected lines in the selection style. The
// plain text is used rather than the styled original because a background
// applied over text that already resets its own colors renders unevenly; a
// flat highlight reads unambiguously as a selection.
func (m Model) applySelection(content string) string {
	lines := strings.Split(content, "\n")
	lo, hi := m.selectionRange()

	for i := lo; i <= hi && i < len(lines); i++ {
		if i < 0 {
			continue
		}
		lines[i] = m.styles.Selection.Render(m.plainLines[i])
	}
	return strings.Join(lines, "\n")
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

// ScrollToPost moves the cursor to the given post and scrolls it into view.
// It reports whether the post is currently loaded; a search can return a hit
// that is older than the window of posts the client holds.
func (m *Model) ScrollToPost(postID string) bool {
	for i, p := range m.posts {
		if p.ID != postID {
			continue
		}
		m.cursor = i
		m.updateContent()
		m.scrollToCursor()
		return true
	}
	return false
}

// SetSearchTerm highlights every occurrence of term across the history, and
// marks one post as the active hit. An empty term clears the highlighting.
func (m *Model) SetSearchTerm(term string) {
	if m.searchTerm == term {
		return
	}
	m.searchTerm = term
	// Highlighting is baked into the rendered text, so the cache has to go.
	m.cache = make(map[string]string)
	m.updateContent()
}

// SearchTerm returns the active highlight term.
func (m Model) SearchTerm() string { return m.searchTerm }

// highlightMatches wraps each case-insensitive occurrence of the search term
// in the match style. It operates on already-rendered text, so it skips
// anything inside an ANSI escape sequence — otherwise a term like "m" would
// match inside a color code and corrupt the output.
func (m Model) highlightMatches(rendered string) string {
	if m.searchTerm == "" {
		return rendered
	}

	term := strings.ToLower(m.searchTerm)
	var b strings.Builder
	lower := strings.ToLower(rendered)

	for i := 0; i < len(rendered); {
		// Copy escape sequences through untouched.
		if rendered[i] == 0x1b {
			end := strings.IndexByte(rendered[i:], 'm')
			if end < 0 {
				b.WriteString(rendered[i:])
				break
			}
			b.WriteString(rendered[i : i+end+1])
			i += end + 1
			continue
		}

		if strings.HasPrefix(lower[i:], term) {
			b.WriteString(m.styles.SearchMatch.Render(rendered[i : i+len(term)]))
			i += len(term)
			continue
		}

		b.WriteByte(rendered[i])
		i++
	}
	return b.String()
}

// --- Selection -------------------------------------------------------------
//
// Selection is line-based rather than character-based. The rendered history is
// markdown-formatted, wrapped, and full of ANSI escapes, so mapping a screen
// column back to an offset in the source text is unreliable; whole lines are
// both predictable and enough to copy a message out.

// SetSelectionAnchor starts a selection at the given content line.
func (m *Model) SetSelectionAnchor(line int) {
	m.selAnchor = clampLine(line, len(m.plainLines))
	m.selHead = m.selAnchor
	m.selActive = true
	m.updateContent()
}

// ExtendSelection moves the free end of the selection, which is what a drag
// does. It is a no-op when no selection has been started.
func (m *Model) ExtendSelection(line int) {
	if !m.selActive {
		return
	}
	m.selHead = clampLine(line, len(m.plainLines))
	m.updateContent()
}

// ClearSelection removes the selection.
func (m *Model) ClearSelection() {
	if !m.selActive {
		return
	}
	m.selActive = false
	m.updateContent()
}

// HasSelection reports whether anything is selected.
func (m Model) HasSelection() bool { return m.selActive }

// SelectedText returns the selected lines as plain text, with styling and
// escape sequences removed so it can go straight to a clipboard.
func (m Model) SelectedText() string {
	if !m.selActive || len(m.plainLines) == 0 {
		return ""
	}
	lo, hi := m.selectionRange()
	return strings.Join(m.plainLines[lo:hi+1], "\n")
}

// selectionRange returns the ordered selection bounds; dragging upwards puts
// the head before the anchor.
func (m Model) selectionRange() (int, int) {
	lo, hi := m.selAnchor, m.selHead
	if lo > hi {
		lo, hi = hi, lo
	}
	return clampLine(lo, len(m.plainLines)), clampLine(hi, len(m.plainLines))
}

// LineAt converts a row within this pane's content area to a content line.
func (m Model) LineAt(contentRow int) int {
	return m.viewport.YOffset + contentRow
}

// SelectPostAtLine moves the cursor to the post occupying a content line and
// reports whether one was found.
func (m *Model) SelectPostAtLine(line int) bool {
	if line < 0 || line >= len(m.lineToPost) {
		return false
	}
	idx := m.lineToPost[line]
	if idx < 0 || idx >= len(m.posts) {
		return false
	}
	m.cursor = idx
	m.updateContent()
	return true
}

func clampLine(v, n int) int {
	switch {
	case n == 0, v < 0:
		return 0
	case v >= n:
		return n - 1
	default:
		return v
	}
}
