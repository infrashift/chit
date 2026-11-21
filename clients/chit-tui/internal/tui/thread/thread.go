// Package thread renders a thread (root post plus replies) as a read-only
// pane. It swaps into the main content area; replies are composed in the
// app's regular input box.
package thread

import (
	"strings"

	bvp "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/post"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// Model is the thread pane component.
type Model struct {
	viewport        bvp.Model
	rootPost        *model.Post
	replies         []*model.Post
	dir             post.Directory
	postTags        map[string][]*model.Tag
	currentUsername string
	focused         bool
	styles          styles.Styles
	width           int
	height          int
	renderer        *glamour.TermRenderer
	cache           map[string]string
}

// New creates a new thread model.
func New(s styles.Styles) Model {
	m := Model{
		viewport: bvp.New(0, 0),
		dir:      post.NewDirectory(),
		styles:   s,
		cache:    make(map[string]string),
	}
	assert.That(m.viewport.Width == 0 && m.viewport.Height == 0, "thread.New: viewport not empty")
	assert.That(len(m.allPosts()) == 0, "thread.New: a new pane shows posts")
	return m
}

// SetThread loads a thread into the pane.
func (m *Model) SetThread(root *model.Post, replies []*model.Post) {
	m.rootPost = root
	m.replies = replies
	m.cache = make(map[string]string)
	m.updateContent()
	assert.That(m.cache != nil, "thread.SetThread: nil render cache")
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.SetThread: cache holds posts not shown")
	assert.That(m.viewport.YOffset >= 0, "thread.SetThread: negative scroll offset")
}

// Clear resets the pane when leaving the thread view.
func (m *Model) Clear() {
	m.rootPost = nil
	m.replies = nil
	m.cache = make(map[string]string)
	m.viewport.SetContent("")
	assert.That(len(m.allPosts()) == 0, "thread.Clear: posts left after clearing")
	assert.That(m.viewport.TotalLineCount() == 1, "thread.Clear: content left after clearing")
	assert.That(m.viewport.YOffset >= 0, "thread.Clear: negative scroll offset")
}

// SetUsernames sets the username map.
func (m *Model) SetUsernames(names map[string]string) {
	assert.That(names != nil, "thread.SetUsernames: nil map")
	next := m.dir
	next.Names = names
	m.setDirectory(next)
}

// SetUserKinds sets the actor types of the agents and bots.
func (m *Model) SetUserKinds(kinds map[string]string) {
	assert.That(kinds != nil, "thread.SetUserKinds: nil map")
	next := m.dir
	next.Kinds = kinds
	m.setDirectory(next)
}

// SetFiles sets the file records that the posts show.
func (m *Model) SetFiles(files map[string]*model.File) {
	assert.That(files != nil, "thread.SetFiles: nil map")
	next := m.dir
	next.Files = files
	m.setDirectory(next)
}

// setDirectory replaces the directory and draws again only the posts that
// show something that changed.
func (m *Model) setDirectory(next post.Directory) {
	assert.That(m.cache != nil, "thread.setDirectory: nil render cache")
	for _, p := range m.allPosts() {
		if m.dir.Shows(p) != next.Shows(p) {
			delete(m.cache, p.ID)
		}
	}
	m.dir = next
	m.updateContent()
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.setDirectory: cache holds posts not shown")
	assert.That(m.viewport.YOffset >= 0, "thread.setDirectory: negative scroll offset")
}

// SetCurrentUsername sets the current user's username for mention highlighting.
func (m *Model) SetCurrentUsername(username string) {
	m.currentUsername = username
	m.cache = make(map[string]string)
	m.updateContent()
	assert.That(m.cache != nil, "thread.SetCurrentUsername: nil render cache")
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.SetCurrentUsername: cache holds posts not shown")
	assert.That(m.viewport.YOffset >= 0, "thread.SetCurrentUsername: negative scroll offset")
}

// RootPost returns the root post of the thread.
func (m Model) RootPost() *model.Post { return m.rootPost }

// Replies returns the reply posts.
func (m Model) Replies() []*model.Post { return m.replies }

// AppendReply adds a reply. Your own reply arrives twice, from the HTTP
// response and the WebSocket echo, so one already shown is ignored.
func (m *Model) AppendReply(p *model.Post) {
	assert.That(p != nil, "thread.AppendReply: nil post")
	assert.That(m.cache != nil, "thread.AppendReply: nil render cache")
	for _, r := range m.replies {
		if r.ID == p.ID {
			return
		}
	}
	m.replies = append(m.replies, p)
	m.updateContent()
	m.viewport.GotoBottom()
	assert.That(m.replies[len(m.replies)-1] == p, "thread.AppendReply: reply not appended last")
	assert.That(m.viewport.YOffset >= 0, "thread.AppendReply: negative scroll offset")
}

// Focus sets focus.
func (m *Model) Focus() {
	m.focused = true
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
}

// Focused returns the focus state.
func (m Model) Focused() bool { return m.focused }

// SetStyles replaces the styles and re-renders all posts.
// Resets the renderer so it picks up the new theme's markdown colors.
func (m *Model) SetStyles(s styles.Styles) {
	m.styles = s
	m.renderer = nil
	m.cache = make(map[string]string)
	m.updateContent()
	assert.That(m.cache != nil, "thread.SetStyles: nil render cache")
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.SetStyles: cache holds posts not shown")
	assert.That(m.viewport.YOffset >= 0, "thread.SetStyles: negative scroll offset")
}

// SetSize sets the pane dimensions. Posts are wrapped to the width, so only
// a width change re-renders them.
func (m *Model) SetSize(w, h int) {
	assert.That(m.viewport.Height >= 0, "thread.SetSize: negative viewport height")
	if w == m.width && h == m.height {
		return
	}
	widthChanged := w != m.width
	m.width = w
	m.height = h
	m.viewport.Width = w - 4
	m.viewport.Height = max(h-2, 3)
	assert.That(m.viewport.Height >= 3, "thread.SetSize: viewport shorter than 3 lines")
	if !widthChanged {
		return
	}
	m.renderer = nil
	m.cache = make(map[string]string)
	m.updateContent()
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.SetSize: cache holds posts not shown")
	assert.That(m.viewport.YOffset >= 0, "thread.SetSize: negative scroll offset")
}

// ScrollBy scrolls the content by the given number of lines (negative = up).
// It works regardless of focus so the mouse wheel can scroll an unfocused pane.
func (m *Model) ScrollBy(lines int) {
	m.viewport.SetYOffset(m.viewport.YOffset + lines)
	assert.That(m.viewport.YOffset >= 0, "thread.ScrollBy: negative scroll offset")
	assert.That(m.viewport.YOffset <= m.viewport.TotalLineCount(), "thread.ScrollBy: scrolled past the content")
}

// Update handles messages (viewport scrolling only; replies are composed in
// the app's input box).
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	assert.That(m.viewport.YOffset >= 0, "thread.Update: negative scroll offset")
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.Update: cache holds posts not shown")
	return m, cmd
}

// View renders the thread pane.
func (m Model) View() string {
	borderStyle := m.styles.ThreadPanel
	if m.focused {
		borderStyle = borderStyle.BorderForeground(m.styles.ActiveBorder.GetBorderBottomForeground())
	}

	assert.That(m.viewport.YOffset >= 0, "thread.View: negative scroll offset")
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.View: cache holds posts not shown")
	return borderStyle.Width(m.width - 2).Height(m.height - 2).Render(m.viewport.View())
}

func (m *Model) allPosts() []*model.Post {
	var all []*model.Post
	if m.rootPost != nil {
		all = append(all, m.rootPost)
	}
	all = append(all, m.replies...)
	assert.That(len(all) >= len(m.replies), "thread.allPosts: replies dropped")
	assert.That(m.rootPost == nil || all[0] == m.rootPost, "thread.allPosts: root post not first")
	return all
}

func (m *Model) ensureRenderer() {
	if m.renderer == nil {
		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(m.styles.MarkdownStyleConfig),
			glamour.WithWordWrap(max(m.width-10, 20)),
			glamour.WithPreservedNewLines(),
			glamour.WithEmoji(),
		)
		if err != nil {
			// The renderer stays nil: posts show as plain text, and the
			// next update tries again.
			errlog.Debug(err, "create thread markdown renderer")
			return
		}
		m.renderer = r
	}
}

func (m *Model) updateContent() {
	assert.That(m.cache != nil, "thread.updateContent: nil render cache")
	assert.That(m.dir.Names != nil, "thread.updateContent: nil username map")
	m.ensureRenderer()
	var lines []string
	if m.rootPost != nil {
		rendered := m.renderPost(m.rootPost, m.width-6)
		lines = append(lines, rendered)
		lines = append(lines, strings.Repeat("─", max(m.width-8, 10)))
	}
	for _, r := range m.replies {
		lines = append(lines, m.renderPost(r, m.width-6))
	}
	assert.That(len(lines) == 2*len(m.allPosts())-len(m.replies),
		"thread.updateContent: one block per post plus the root's separator")
	m.viewport.SetContent(strings.Join(lines, "\n\n"))
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.updateContent: cache holds posts not shown")
	assert.That(m.viewport.YOffset >= 0, "thread.updateContent: negative scroll offset")
}

// SetPostTags supplies tags per post ID. Without them the thread pane renders
// posts untagged, so a post's tags appear in the channel and then vanish when
// the same post is opened as a thread.
func (m *Model) SetPostTags(tags map[string][]*model.Tag) {
	assert.That(tags != nil, "thread.SetPostTags: nil map")
	m.postTags = tags
	m.cache = make(map[string]string)
	m.updateContent()
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.SetPostTags: cache holds posts not shown")
	assert.That(m.viewport.YOffset >= 0, "thread.SetPostTags: negative scroll offset")
}

// tagNames returns the tag names for a post, or nil when it has none.
func (m *Model) tagNames(postID string) []string {
	tags := m.postTags[postID]
	if len(tags) == 0 {
		return nil
	}
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	assert.That(len(names) == len(tags), "thread.tagNames: one name per tag")
	return names
}

func (m *Model) renderPost(p *model.Post, width int) string {
	assert.That(p != nil, "thread.renderPost: nil post")
	assert.That(m.cache != nil, "thread.renderPost: nil render cache")
	if cached, ok := m.cache[p.ID]; ok {
		return cached
	}
	username := m.dir.Username(p.UserID)
	pb := post.New(p, username, m.styles, width, m.renderer, m.currentUsername, 0, m.tagNames(p.ID)).With(m.dir.Extra(p))
	rendered := pb.View()
	assert.That(username != "" || p.UserID == "", "thread.renderPost: known author left unnamed")
	m.cache[p.ID] = rendered
	assert.That(len(m.cache) <= len(m.allPosts()), "thread.renderPost: cache holds posts not shown")
	return rendered
}

// RemoveReply drops a deleted reply, and reports whether it was shown.
func (m *Model) RemoveReply(id string) bool {
	assert.That(m.cache != nil, "thread.RemoveReply: nil render cache")
	before := len(m.replies)
	for i, r := range m.replies {
		if r.ID == id {
			m.replies = append(m.replies[:i], m.replies[i+1:]...)
			delete(m.cache, id)
			m.updateContent()
			assert.That(len(m.replies) == before-1, "thread.RemoveReply: not exactly one reply removed")
			assert.That(len(m.cache) <= len(m.allPosts()), "thread.RemoveReply: cache holds posts not shown")
			return true
		}
	}
	return false
}

// UpdatePost replaces the root post or one of the replies in place, and
// reports whether it was present. An edit to a post being read in a thread
// should show there too, not only in the channel.
func (m *Model) UpdatePost(p *model.Post) bool {
	assert.That(p != nil, "thread.UpdatePost: nil post")
	before := len(m.replies)
	if m.rootPost != nil && m.rootPost.ID == p.ID {
		m.rootPost = p
		delete(m.cache, p.ID)
		m.updateContent()
		assert.That(len(m.replies) == before, "thread.UpdatePost: an edit to the root changed the replies")
		return true
	}
	for i, reply := range m.replies {
		if reply.ID != p.ID {
			continue
		}
		m.replies[i] = p
		delete(m.cache, p.ID)
		m.updateContent()
		assert.That(len(m.replies) == before, "thread.UpdatePost: an edit changed the reply count")
		return true
	}
	return false
}
