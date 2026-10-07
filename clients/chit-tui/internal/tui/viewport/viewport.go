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
	return Model{
		viewport:     bvp.New(0, 0),
		usernames:    make(map[string]string),
		threadCounts: make(map[string]int),
		postTags:     make(map[string][]string),
		styles:       s,
		cache:        make(map[string]renderedPost),
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
	m.cache = make(map[string]renderedPost)
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
	m.cache = make(map[string]renderedPost)
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
	m.SetPostsTags(map[string][]string{postID: tagNames})
}

// SetPostsTags updates the tag names for many posts at once. Tags arrive a
// page at a time, and setting them one by one redrew the whole history for
// each post.
func (m *Model) SetPostsTags(tags map[string][]string) {
	for id, names := range tags {
		m.postTags[id] = names
		delete(m.cache, id)
	}
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
	m.cache = make(map[string]renderedPost)
	m.updateContent()
}

// Focus sets focus state and re-renders to show the selection highlight.
func (m *Model) Focus() {
	if m.focused {
		return
	}
	m.focused = true
	m.updateContent()
}

// Blur removes focus and re-renders to remove the selection highlight.
func (m *Model) Blur() {
	if !m.focused {
		return
	}
	m.focused = false
	m.updateContent()
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
		return
	}
	m.renderer = nil
	m.cache = make(map[string]renderedPost)
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
	}
	content := strings.Join(blocks, "\n\n")
	m.plainLines = plain
	m.indexLineOwners()

	if m.selActive {
		content = m.applySelection(content)
	}
	m.viewport.SetContent(content)
}

// render returns a post as drawn, from the cache when it is there, with
// the current search term highlighted.
func (m *Model) render(p *model.Post) renderedPost {
	rp, ok := m.cache[p.ID]
	if !ok {
		username := m.usernames[p.UserID]
		if username == "" {
			username = p.UserID[:min(8, len(p.UserID))]
		}
		pb := post.New(p, username, m.styles, m.width-4, m.renderer, m.currentUsername, m.threadCounts[p.ID], m.postTagNames(p.ID))
		rp.text = pb.View()
		rp.plain = plainLinesOf(rp.text)
	}
	if m.searchTerm != "" && (rp.hlTerm != m.searchTerm || rp.hlText == "") {
		rp.hlTerm, rp.hlText = m.searchTerm, m.highlightMatches(rp.text)
	}
	m.cache[p.ID] = rp
	return rp
}

// plainLinesOf splits rendered text into lines without ANSI escapes.
func plainLinesOf(s string) []string {
	return strings.Split(ansi.Strip(s), "\n")
}

// indexLineOwners maps each content line to the post it belongs to: from a
// post's first line up to the next post's, which takes in the blank line
// and any day separator after it. The reverse map is what turns a mouse row
// into a post, which post→line offsets alone cannot do.
func (m *Model) indexLineOwners() {
	m.lineToPost = make([]int, len(m.plainLines))
	for i := range m.lineToPost {
		m.lineToPost[i] = -1
	}
	for i, start := range m.postLineOffsets {
		end := len(m.lineToPost)
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
	term := []rune(m.searchTerm)

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
			b.WriteString(m.styles.SearchMatch.Render(rendered[i : i+n]))
			i += n
			continue
		}

		_, size := utf8.DecodeRuneInString(rendered[i:])
		b.WriteString(rendered[i : i+size])
		i += size
	}
	return b.String()
}

// matchFold reports how many bytes at the start of s match term, ignoring
// case, or 0 if they do not. It compares character by character, because a
// lowercased copy does not keep byte offsets: some characters change length.
func matchFold(s string, term []rune) int {
	n := 0
	for _, want := range term {
		if n >= len(s) {
			return 0
		}
		got, size := utf8.DecodeRuneInString(s[n:])
		if got != want && !strings.EqualFold(string(got), string(want)) {
			return 0
		}
		n += size
	}
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
