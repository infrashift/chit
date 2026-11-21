package viewport

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	bvp "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
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
	dir             post.Directory
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
	loading         bool
	searchTerm      string
	focused         bool
	styles          styles.Styles
	width           int
	height          int
	renderer        *glamour.TermRenderer
	cache           map[string]renderedPost // by post ID
}

// renderedPost is a post as drawn, kept until something it depends on
// changes. Rendering through glamour is by far the costliest step, and the
// plain lines spare re-stripping the whole history on every cursor move.
type renderedPost struct {
	text  string
	plain []string // text without ANSI escapes, one entry per line
	// hlTerm and hlText cache text with hlTerm highlighted.
	hlTerm string
	hlText string
}

// New creates a new viewport model.
func New(s styles.Styles) Model {
	assert.That(s.Viewport.GetBorderStyle().Left != "", "viewport.New: styles not built by styles.New")
	assert.That(s.SelectedPost.GetPaddingLeft() > 0, "viewport.New: styles without the selected-post padding")
	m := Model{
		viewport:     bvp.New(0, 0),
		dir:          post.NewDirectory(),
		threadCounts: make(map[string]int),
		postTags:     make(map[string][]string),
		// No post is selected until one is loaded.
		cursor: -1,
		styles: s,
		cache:  make(map[string]renderedPost),
	}
	assert.That(m.viewport.YOffset == 0, "viewport.New: a fresh viewport is scrolled")
	return m
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
	m.cache = make(map[string]renderedPost)
	m.updateContent()
	m.viewport.GotoBottom()
	assert.That(len(m.posts) == len(posts), "Model.SetPosts: posts lost in the reversal")
	assert.That(m.cursor == len(m.posts)-1, "Model.SetPosts: the newest post is not selected")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.SetPosts: a line offset for each post")
	assert.That(m.viewport.YOffset >= 0, "Model.SetPosts: negative scroll offset")
	assert.That(len(m.cache) <= len(m.posts), "Model.SetPosts: cache holds posts that are not shown")
}

// AppendPost adds a post at the end. A channel that started empty has no
// selection yet (cursor -1); select the first post that arrives so
// Enter/`t` work without reloading the channel.
func (m *Model) AppendPost(p *model.Post) {
	assert.That(m.cursor >= -1, "Model.AppendPost: cursor below -1")
	m.posts = append(m.posts, p)
	if m.cursor < 0 {
		m.cursor = len(m.posts) - 1
	}
	m.updateContent()
	m.viewport.GotoBottom()
	assert.That(m.cursor >= 0 && m.cursor < len(m.posts), "Model.AppendPost: cursor outside the posts")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.AppendPost: a line offset for each post")
	assert.That(len(m.cache) <= len(m.posts), "Model.AppendPost: cache holds posts that are not shown")
}

// HasPost reports whether a post is already displayed. Both the HTTP response
// to sending and the WebSocket echo carry the same post, so whichever arrives
// second must not duplicate it.
func (m Model) HasPost(id string) bool {
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.HasPost: a line offset for each post")
	assert.That(m.cursor >= -1, "Model.HasPost: cursor below -1")
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
	before := len(m.posts)
	for i, existing := range m.posts {
		if existing.ID != p.ID {
			continue
		}
		m.posts[i] = p
		delete(m.cache, p.ID)
		m.updateContent()
		assert.That(len(m.posts) == before, "Model.UpdatePost: the post count changed")
		assert.That(len(m.postLineOffsets) == len(m.posts), "Model.UpdatePost: a line offset for each post")
		assert.That(len(m.cache) <= len(m.posts), "Model.UpdatePost: cache holds posts that are not shown")
		return true
	}
	return false
}

// SetPinned flips a post's pinned flag in place and reports whether it was
// present. The pin events carry only an ID, not the post, so there is nothing
// to replace it with.
func (m *Model) SetPinned(id string, pinned bool) bool {
	for _, p := range m.posts {
		if p.ID != id {
			continue
		}
		p.IsPinned = pinned
		delete(m.cache, id)
		m.updateContent()
		assert.That(len(m.postLineOffsets) == len(m.posts), "Model.SetPinned: a line offset for each post")
		assert.That(len(m.lineToPost) == len(m.plainLines), "Model.SetPinned: an owner for each line")
		return true
	}
	return false
}

// RemovePost drops a post from the history and reports whether it was present.
func (m *Model) RemovePost(id string) bool {
	before := len(m.posts)
	for i, p := range m.posts {
		if p.ID != id {
			continue
		}
		m.posts = append(m.posts[:i], m.posts[i+1:]...)
		delete(m.cache, id)
		// A post removed above the cursor shifts the selected one up a
		// place; follow it rather than land on its neighbor.
		if i < m.cursor {
			m.cursor--
		}
		if m.cursor >= len(m.posts) {
			m.cursor = len(m.posts) - 1
		}
		m.updateContent()
		assert.That(len(m.posts) == before-1, "Model.RemovePost: not exactly one post removed")
		assert.That(m.cursor >= -1 && m.cursor < len(m.posts), "Model.RemovePost: cursor outside the posts")
		assert.That(len(m.postLineOffsets) == len(m.posts), "Model.RemovePost: a line offset for each post")
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
	// it so the combined list stays chronological. Paging is by offset, so
	// posts that arrived since the first page shift the boundary and the
	// page can repeat some of what is shown; those are skipped.
	shown := make(map[string]bool, len(m.posts))
	for _, p := range m.posts {
		shown[p.ID] = true
	}
	reversed := make([]*model.Post, 0, len(older))
	for i := len(older) - 1; i >= 0; i-- {
		if !shown[older[i].ID] {
			reversed = append(reversed, older[i])
		}
	}
	if len(reversed) == 0 {
		return
	}
	assert.That(len(reversed) <= len(older), "Model.PrependPosts: more posts than were given")
	assert.That(m.cursor >= -1, "Model.PrependPosts: cursor below -1")

	linesBefore := len(m.plainLines)
	postsBefore := len(m.posts)
	m.posts = append(reversed, m.posts...)
	m.cursor += len(reversed)
	m.updateContent()
	assert.That(len(m.posts) == postsBefore+len(reversed), "Model.PrependPosts: posts lost")
	assert.That(m.cursor >= 0 && m.cursor < len(m.posts), "Model.PrependPosts: cursor outside the posts")
	assert.That(len(m.plainLines) >= linesBefore, "Model.PrependPosts: content shrank")

	// Hold the reader's position: everything they were looking at has moved
	// down by however many lines were inserted above it.
	m.viewport.SetYOffset(m.viewport.YOffset + (len(m.plainLines) - linesBefore))
	assert.That(m.viewport.YOffset >= 0, "Model.PrependPosts: negative scroll offset")
}

// AtTop reports whether the view is scrolled to the oldest loaded post, which
// is when there is any point fetching more.
func (m Model) AtTop() bool {
	assert.That(m.viewport.YOffset >= 0, "Model.AtTop: negative scroll offset")
	return m.viewport.YOffset == 0
}

// SetUsernames updates the username map and re-renders only posts whose
// displayed username actually changed.
func (m *Model) SetUsernames(names map[string]string) {
	assert.That(names != nil, "Model.SetUsernames: nil map")
	next := m.dir
	next.Names = names
	m.setDirectory(next)
}

// SetUserKinds sets the actor types of the agents and bots.
func (m *Model) SetUserKinds(kinds map[string]string) {
	assert.That(kinds != nil, "Model.SetUserKinds: nil map")
	next := m.dir
	next.Kinds = kinds
	m.setDirectory(next)
}

// SetFiles sets the file records that the posts show.
func (m *Model) SetFiles(files map[string]*model.File) {
	assert.That(files != nil, "Model.SetFiles: nil map")
	next := m.dir
	next.Files = files
	m.setDirectory(next)
}

// setDirectory replaces the directory and re-renders only the posts that
// show something that changed: a name, an actor type, a file record.
func (m *Model) setDirectory(next post.Directory) {
	for _, p := range m.posts {
		if m.dir.Shows(p) != next.Shows(p) {
			delete(m.cache, p.ID)
		}
	}
	m.dir = next
	m.updateContent()
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.setDirectory: a line offset for each post")
	assert.That(len(m.cache) <= len(m.posts), "Model.setDirectory: cache holds posts that are not shown")
}

// SetCurrentUsername sets the current user's username for mention highlighting.
func (m *Model) SetCurrentUsername(username string) {
	m.currentUsername = username
	m.cache = make(map[string]renderedPost)
	m.updateContent()
	assert.That(len(m.cache) <= len(m.posts), "Model.SetCurrentUsername: cache holds posts that are not shown")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.SetCurrentUsername: a line offset for each post")
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.SetCurrentUsername: an owner for each line")
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
	assert.That(len(m.threadCounts) == len(counts), "Model.SetThreadCounts: the copy lost counts")
	m.updateContent()
	assert.That(len(m.cache) <= len(m.posts), "Model.SetThreadCounts: cache holds posts that are not shown")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.SetThreadCounts: a line offset for each post")
}

// SetPostTags updates the tag names for a post and invalidates its cache.
func (m *Model) SetPostTags(postID string, tagNames []string) {
	assert.That(m.postTags != nil, "Model.SetPostTags: postTags not made by New")
	m.SetPostsTags(map[string][]string{postID: tagNames})
	assert.That(len(m.postTags) >= 1, "Model.SetPostTags: tags not stored")
}

// SetPostsTags updates the tag names for many posts at once. Tags arrive a
// page at a time, and setting them one by one redrew the whole history for
// each post.
func (m *Model) SetPostsTags(tags map[string][]string) {
	assert.That(m.postTags != nil, "Model.SetPostsTags: postTags not made by New")
	for id, names := range tags {
		m.postTags[id] = names
		delete(m.cache, id)
	}
	assert.That(len(m.postTags) >= len(tags), "Model.SetPostsTags: tags not stored")
	assert.That(m.cache != nil, "Model.SetPostsTags: cache not made by New")
	m.updateContent()
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.SetPostsTags: a line offset for each post")
}

func (m Model) postTagNames(postID string) []string {
	assert.That(m.postTags != nil, "Model.postTagNames: postTags not made by New")
	return m.postTags[postID]
}

// Posts returns the current posts.
func (m Model) Posts() []*model.Post { return m.posts }

// SelectedPost returns the post under the cursor.
func (m Model) SelectedPost() *model.Post {
	assert.That(m.cursor >= -1, "Model.SelectedPost: cursor below -1")
	assert.That(m.cursor < len(m.posts) || len(m.posts) == 0, "Model.SelectedPost: cursor past the posts")
	if m.cursor >= 0 && m.cursor < len(m.posts) {
		return m.posts[m.cursor]
	}
	return nil
}

// SetStyles replaces the styles and re-renders all posts.
// Resets the renderer so it picks up the new theme's markdown colors.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.Viewport.GetBorderStyle().Left != "", "Model.SetStyles: styles not built by styles.New")
	m.styles = s
	m.renderer = nil
	m.cache = make(map[string]renderedPost)
	m.updateContent()
	assert.That(len(m.cache) <= len(m.posts), "Model.SetStyles: cache holds posts that are not shown")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.SetStyles: a line offset for each post")
}

// Focus sets focus state and re-renders to show the selection highlight.
func (m *Model) Focus() {
	if m.focused {
		return
	}
	m.focused = true
	m.updateContent()
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.Focus: an owner for each line")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.Focus: a line offset for each post")
}

// Blur removes focus and re-renders to remove the selection highlight.
func (m *Model) Blur() {
	if !m.focused {
		return
	}
	m.focused = false
	m.updateContent()
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.Blur: an owner for each line")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.Blur: a line offset for each post")
}

// Focused returns the focus state.
func (m Model) Focused() bool { return m.focused }

// SetSize sets the viewport dimensions. Posts are wrapped to the width, so
// only a width change re-renders them; the root model re-applies its layout
// on every channel switch and thread open, nearly always unchanged.
func (m *Model) SetSize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	widthChanged := w != m.width
	m.width = w
	m.height = h
	m.viewport.Width = w - 2
	m.viewport.Height = h - 2
	if !widthChanged {
		m.viewport.SetYOffset(m.viewport.YOffset) // re-clamp to the new height
		assert.That(m.viewport.YOffset >= 0, "Model.SetSize: negative scroll offset")
		return
	}
	m.renderer = nil
	m.cache = make(map[string]renderedPost)
	m.updateContent()
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.SetSize: an owner for each line")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.SetSize: a line offset for each post")
}

// ScrollBy scrolls the content by the given number of lines (negative = up).
// It works regardless of focus so the mouse wheel can scroll an unfocused pane.
func (m *Model) ScrollBy(lines int) {
	assert.That(m.viewport.YOffset >= 0, "Model.ScrollBy: negative scroll offset before")
	m.viewport.SetYOffset(m.viewport.YOffset + lines)
	assert.That(m.viewport.YOffset >= 0, "Model.ScrollBy: negative scroll offset after")
}

// Bindings owned by this pane. They are exported so the root model's help
// overlay can describe the same keys the pane actually handles, rather than a
// hand-maintained copy that can drift.

// ReplyKey opens the selected post as a thread, which is how a reply is
// written.
func ReplyKey() key.Binding {
	b := key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "reply in thread"),
	)
	assert.That(b.Enabled() && len(b.Keys()) == 1, "ReplyKey: binding without its key")
	return b
}

// PrevPostKey moves the post cursor up.
func PrevPostKey() key.Binding {
	b := key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "previous post"),
	)
	assert.That(b.Enabled() && len(b.Keys()) == 2, "PrevPostKey: binding without its keys")
	return b
}

// NextPostKey moves the post cursor down.
func NextPostKey() key.Binding {
	b := key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "next post"),
	)
	assert.That(b.Enabled() && len(b.Keys()) == 2, "NextPostKey: binding without its keys")
	return b
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	assert.That(m.cursor >= -1, "Model.Update: cursor below -1")

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(keyMsg, PrevPostKey()):
		if m.cursor > 0 {
			m.cursor--
			assert.That(m.cursor < len(m.posts), "Model.Update: cursor past the posts after moving up")
			m.updateContent()
			m.scrollToCursor()
		}
		return m, nil
	case key.Matches(keyMsg, NextPostKey()):
		if m.cursor < len(m.posts)-1 {
			m.cursor++
			assert.That(m.cursor >= 0 && m.cursor < len(m.posts), "Model.Update: cursor outside the posts after moving down")
			m.updateContent()
			m.scrollToCursor()
		}
		return m, nil
	case key.Matches(keyMsg, ReplyKey()):
		p := m.SelectedPost()
		if p != nil {
			assert.That(m.cursor >= 0 && m.cursor < len(m.posts), "Model.Update: selected post outside the cursor range")
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

	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.View: a line offset for each post")
	assert.That(m.cursor >= -1, "Model.View: cursor below -1")
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.View: an owner for each line")
	body := m.viewport.View()
	// An empty pane is ambiguous: still loading, or nothing to show? Say
	// which, rather than rendering a blank bordered box.
	if len(m.posts) == 0 {
		body = m.styles.DaySeparator.Render(m.placeholder())
	}

	return borderStyle.Width(m.width - 2).Height(m.height - 2).Render(body)
}

// placeholder is the text shown when there are no posts to render.
func (m Model) placeholder() string {
	if m.loading {
		return "Loading messages…"
	}
	return "No messages yet — type below to start the conversation."
}

// SetLoading marks a fetch as in flight, so the pane can say so instead of
// showing a stale or blank view while the request is out.
func (m *Model) SetLoading(loading bool) { m.loading = loading }

// Loading reports whether the channel's first page is still on its way.
func (m Model) Loading() bool { return m.loading }

// Note: a "loading older" banner is deliberately not injected into the
// rendered content. Every line of that content is indexed for selection and
// click-to-select, so an extra line would shift the map and make clicks
// resolve to the wrong post. The status line carries that notice instead.

func (m *Model) ensureRenderer() {
	if m.renderer == nil {
		wrap := max(m.width-8, 20)
		assert.That(wrap >= 20 && wrap >= m.width-8, "Model.ensureRenderer: wrap width below its floor")
		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(m.styles.MarkdownStyleConfig),
			glamour.WithWordWrap(wrap),
			glamour.WithPreservedNewLines(),
			glamour.WithEmoji(),
		)
		if err != nil {
			// The renderer stays nil: posts show as plain text, and the
			// next update tries again.
			errlog.Debug(err, "create channel markdown renderer")
			return
		}
		m.renderer = r
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
	assert.That(labelWidth > 0, "formatDaySeparator: empty label")
	remaining := width - labelWidth
	if remaining < 2 {
		return label
	}
	left := remaining / 2
	right := remaining - left
	assert.That(left >= 1 && right >= left, "formatDaySeparator: rule halves out of balance")
	assert.That(left+right+labelWidth == width, "formatDaySeparator: separator not the given width")
	return strings.Repeat("─", left) + label + strings.Repeat("─", right)
}

func (m *Model) updateContent() {
	m.ensureRenderer()
	// Blocks are the day separators and posts, joined by a blank line.
	// plain mirrors the joined content line for line, without escapes, and
	// is assembled from each post's cached plain lines rather than by
	// stripping the whole history again.
	blocks := make([]string, 0, len(m.posts)+8)
	plain := make([]string, 0, len(m.plainLines)+8)
	add := func(display string, lines []string) int {
		if len(blocks) > 0 {
			plain = append(plain, "")
		}
		first := len(plain)
		blocks = append(blocks, display)
		plain = append(plain, lines...)
		return first
	}

	assert.That(m.cache != nil, "Model.updateContent: cache not made by New")
	m.postLineOffsets = make([]int, len(m.posts))
	var prevDate time.Time
	for i, p := range m.posts {
		curDate := model.MillisToTime(p.CreateAt)
		if i > 0 && !sameDay(prevDate, curDate) {
			sep := m.styles.DaySeparator.Render(formatDaySeparator(curDate, m.width-4))
			add(sep, plainLinesOf(sep))
		}
		prevDate = curDate

		rp := m.render(p)
		display, lines := rp.text, rp.plain
		assert.That(len(lines) >= 1, "Model.updateContent: a post with no lines")
		if m.searchTerm != "" {
			// Highlighting changes the escapes, never the text, so the
			// plain lines still hold.
			display = rp.hlText
		}
		if i == m.cursor && m.focused {
			display = m.styles.SelectedPost.Render(display)
			lines = plainLinesOf(display)
		}
		m.postLineOffsets[i] = add(display, lines)
		assert.That(m.postLineOffsets[i] < len(plain), "Model.updateContent: post starts past the content")
		assert.That(i == 0 || m.postLineOffsets[i] > m.postLineOffsets[i-1], "Model.updateContent: post offsets not increasing")
	}
	content := strings.Join(blocks, "\n\n")
	assert.That(len(blocks) == 0 || strings.Count(content, "\n")+1 == len(plain),
		"Model.updateContent: plain lines out of step with the content")
	m.plainLines = plain
	m.indexLineOwners()

	if m.selActive {
		content = m.applySelection(content)
	}
	m.viewport.SetContent(content)
	assert.That(len(m.cache) <= len(m.posts), "Model.updateContent: cache holds posts that are not shown")
}

// render returns a post as drawn, from the cache when it is there, with
// the current search term highlighted.
func (m *Model) render(p *model.Post) renderedPost {
	assert.That(m.cache != nil, "Model.render: cache not made by New")
	rp, ok := m.cache[p.ID]
	if !ok {
		username := m.dir.Username(p.UserID)
		pb := post.New(p, username, m.styles, m.width-4, m.renderer, m.currentUsername, m.threadCounts[p.ID], m.postTagNames(p.ID)).With(m.dir.Extra(p))
		rp.text = pb.View()
		rp.plain = plainLinesOf(rp.text)
	}
	if m.searchTerm != "" && (rp.hlTerm != m.searchTerm || rp.hlText == "") {
		rp.hlTerm, rp.hlText = m.searchTerm, m.highlightMatches(rp.text)
	}
	assert.That(len(rp.plain) >= 1, "Model.render: a rendered post has no lines")
	assert.That(m.searchTerm == "" || rp.hlTerm == m.searchTerm, "Model.render: highlight is for another term")
	m.cache[p.ID] = rp
	return rp
}

// plainLinesOf splits rendered text into lines without ANSI escapes.
func plainLinesOf(s string) []string {
	lines := strings.Split(ansi.Strip(s), "\n")
	assert.That(len(lines) >= 1, "plainLinesOf: no lines")
	return lines
}

// indexLineOwners maps each content line to the post it belongs to: from a
// post's first line up to the next post's, which takes in the blank line
// and any day separator after it. The reverse map is what turns a mouse row
// into a post, which post→line offsets alone cannot do.
func (m *Model) indexLineOwners() {
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.indexLineOwners: a line offset for each post")
	m.lineToPost = make([]int, len(m.plainLines))
	for i := range m.lineToPost {
		m.lineToPost[i] = -1
	}
	for i, start := range m.postLineOffsets {
		end := len(m.lineToPost)
		if i+1 < len(m.postLineOffsets) {
			end = m.postLineOffsets[i+1]
		}
		assert.That(start >= 0 && start <= end, "Model.indexLineOwners: post lines out of order")
		assert.That(i < len(m.posts), "Model.indexLineOwners: owner past the posts")
		for line := start; line < end && line < len(m.lineToPost); line++ {
			if line >= 0 {
				m.lineToPost[line] = i
			}
		}
	}
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.indexLineOwners: an owner for each line")
}

// applySelection re-renders the selected lines in the selection style. The
// plain text is used rather than the styled original because a background
// applied over text that already resets its own colors renders unevenly; a
// flat highlight reads unambiguously as a selection.
func (m Model) applySelection(content string) string {
	lines := strings.Split(content, "\n")
	lo, hi := m.selectionRange()
	assert.That(lo >= 0 && lo <= hi, "Model.applySelection: selection range out of order")
	assert.That(len(m.plainLines) == 0 || len(lines) == len(m.plainLines),
		"Model.applySelection: content and plain lines out of step")

	// A selection can outlive its posts (an emptied channel), so the loop
	// stops at the end of either slice.
	for i := lo; i <= hi && i < len(lines) && i < len(m.plainLines); i++ {
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
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.scrollToCursor: a line offset for each post")
	target := m.postLineOffsets[m.cursor]
	assert.That(target >= 0 && target < len(m.plainLines), "Model.scrollToCursor: post line past the content")
	if target < m.viewport.YOffset {
		m.viewport.SetYOffset(target)
	} else if target >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(target - m.viewport.Height + 3)
	}
	assert.That(m.viewport.YOffset >= 0, "Model.scrollToCursor: negative scroll offset")
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
		assert.That(m.cursor == i && m.cursor < len(m.posts), "ScrollToPost: cursor not on the post")
		assert.That(m.SelectedPost() == p, "ScrollToPost: another post selected")
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
	m.updateContent()
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.SetSearchTerm: a line offset for each post")
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.SetSearchTerm: an owner for each line")
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
	term := []rune(m.searchTerm)
	assert.That(len(term) > 0, "Model.highlightMatches: empty term")

	var b strings.Builder
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

		if n := matchFold(rendered[i:], term); n > 0 {
			assert.That(i+n <= len(rendered), "Model.highlightMatches: match runs past the text")
			b.WriteString(m.styles.SearchMatch.Render(rendered[i : i+n]))
			i += n
			continue
		}

		_, size := utf8.DecodeRuneInString(rendered[i:])
		assert.That(size >= 1, "Model.highlightMatches: no progress")
		b.WriteString(rendered[i : i+size])
		i += size
	}
	assert.That(b.Len() >= len(rendered), "Model.highlightMatches: output lost text")
	return b.String()
}

// matchFold reports how many bytes at the start of s match term, ignoring
// case, or 0 if they do not. It compares character by character, because a
// lowercased copy does not keep byte offsets: some characters change length.
func matchFold(s string, term []rune) int {
	assert.That(len(term) > 0, "matchFold: empty term")
	n := 0
	for _, want := range term {
		if n >= len(s) {
			return 0
		}
		got, size := utf8.DecodeRuneInString(s[n:])
		assert.That(size >= 1, "matchFold: no progress")
		if got != want && !strings.EqualFold(string(got), string(want)) {
			return 0
		}
		n += size
	}
	assert.That(n > 0 && n <= len(s), "matchFold: match length outside the text")
	return n
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
	assert.That(m.selAnchor >= 0, "Model.SetSelectionAnchor: negative anchor after clamping")
	assert.That(m.selAnchor < len(m.plainLines) || len(m.plainLines) == 0, "Model.SetSelectionAnchor: anchor past the content")
	m.selActive = true
	m.updateContent()
}

// ExtendSelection moves the free end of the selection, which is what a drag
// does. It is a no-op when no selection has been started.
func (m *Model) ExtendSelection(line int) {
	if !m.selActive {
		return
	}
	assert.That(m.selAnchor >= 0, "Model.ExtendSelection: negative anchor")
	m.selHead = clampLine(line, len(m.plainLines))
	assert.That(m.selHead >= 0, "Model.ExtendSelection: negative head after clamping")
	assert.That(m.selHead < len(m.plainLines) || len(m.plainLines) == 0, "Model.ExtendSelection: head past the content")
	m.updateContent()
}

// ClearSelection removes the selection.
func (m *Model) ClearSelection() {
	if !m.selActive {
		return
	}
	m.selActive = false
	m.updateContent()
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.ClearSelection: an owner for each line")
	assert.That(len(m.postLineOffsets) == len(m.posts), "Model.ClearSelection: a line offset for each post")
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
	assert.That(lo >= 0 && lo <= hi, "Model.SelectedText: selection range out of order")
	assert.That(hi < len(m.plainLines), "Model.SelectedText: selection past the content")
	return strings.Join(m.plainLines[lo:hi+1], "\n")
}

// selectionRange returns the ordered selection bounds; dragging upwards puts
// the head before the anchor.
func (m Model) selectionRange() (int, int) {
	lo, hi := m.selAnchor, m.selHead
	if lo > hi {
		lo, hi = hi, lo
	}
	lo, hi = clampLine(lo, len(m.plainLines)), clampLine(hi, len(m.plainLines))
	assert.That(lo <= hi, "Model.selectionRange: bounds out of order after clamping")
	assert.That(lo >= 0, "Model.selectionRange: negative bound after clamping")
	return lo, hi
}

// LineAt converts a row within this pane's content area to a content line.
func (m Model) LineAt(contentRow int) int {
	assert.That(contentRow >= 0, "Model.LineAt: negative content row")
	assert.That(m.viewport.YOffset >= 0, "Model.LineAt: negative scroll offset")
	return m.viewport.YOffset + contentRow
}

// SelectPostAtLine moves the cursor to the post occupying a content line and
// reports whether one was found.
func (m *Model) SelectPostAtLine(line int) bool {
	assert.That(len(m.lineToPost) == len(m.plainLines), "Model.SelectPostAtLine: an owner for each line before")
	if line < 0 || line >= len(m.lineToPost) {
		return false
	}
	idx := m.lineToPost[line]
	if idx < 0 || idx >= len(m.posts) {
		return false
	}
	m.cursor = idx
	m.updateContent()
	assert.That(m.cursor >= 0 && m.cursor < len(m.posts), "Model.SelectPostAtLine: cursor outside the posts")
	assert.That(m.lineToPost[line] == m.cursor, "Model.SelectPostAtLine: line not owned by the selected post")
	return true
}

func clampLine(v, n int) int {
	assert.That(n >= 0, "clampLine: negative line count")
	out := v
	switch {
	case n == 0, v < 0:
		out = 0
	case v >= n:
		out = n - 1
	}
	assert.That(out >= 0 && (out < n || n == 0), "clampLine: result outside the lines")
	return out
}
