package thread

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	bvp "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/post"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// ReplyMsg is sent when a reply is composed.
type ReplyMsg struct {
	RootID  string
	Content string
}

// Model is the thread panel component.
type Model struct {
	viewport        bvp.Model
	input           textarea.Model
	rootPost        *model.Post
	replies         []*model.Post
	usernames       map[string]string
	currentUsername string
	visible         bool
	focused         bool
	styles          styles.Styles
	width           int
	height          int
	renderer        *glamour.TermRenderer
	cache           map[string]string
}

// New creates a new thread model.
func New(s styles.Styles) Model {
	ta := textarea.New()
	ta.Placeholder = "Reply..."
	ta.CharLimit = 65535
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	return Model{
		viewport:  bvp.New(0, 0),
		input:     ta,
		usernames: make(map[string]string),
		styles:    s,
		cache:     make(map[string]string),
	}
}

// SetThread loads a thread into the panel.
func (m *Model) SetThread(root *model.Post, replies []*model.Post) {
	m.rootPost = root
	m.replies = replies
	m.visible = true
	m.cache = make(map[string]string)
	m.updateContent()
}

// SetUsernames sets the username map.
func (m *Model) SetUsernames(names map[string]string) {
	for _, p := range m.allPosts() {
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

// RootPost returns the root post of the thread.
func (m Model) RootPost() *model.Post { return m.rootPost }

// Replies returns the reply posts.
func (m Model) Replies() []*model.Post { return m.replies }

// AppendReply adds a reply.
func (m *Model) AppendReply(p *model.Post) {
	m.replies = append(m.replies, p)
	m.updateContent()
	m.viewport.GotoBottom()
}

// Toggle toggles visibility.
func (m *Model) Toggle() { m.visible = !m.visible }

// Visible returns visibility state.
func (m Model) Visible() bool { return m.visible }

// SetVisible sets visibility.
func (m *Model) SetVisible(v bool) { m.visible = v }

// Focus sets focus.
func (m *Model) Focus() tea.Cmd {
	m.focused = true
	return m.input.Focus()
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
	m.input.Blur()
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
}

// SetSize sets the panel dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	inputHeight := 5
	m.viewport.Width = w - 4
	m.viewport.Height = max(h-inputHeight-4, 3)
	m.input.SetWidth(w - 4)
	m.input.SetHeight(3)
	m.renderer = nil
	m.cache = make(map[string]string)
	m.updateContent()
}

// ScrollBy scrolls the content by the given number of lines (negative = up).
// It works regardless of focus so the mouse wheel can scroll an unfocused pane.
func (m *Model) ScrollBy(lines int) {
	m.viewport.SetYOffset(m.viewport.YOffset + lines)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused || !m.visible {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok && keyMsg.Type == tea.KeyEnter {
		content := strings.TrimSpace(m.input.Value())
		if content != "" && m.rootPost != nil {
			m.input.Reset()
			rootID := m.rootPost.ID
			return m, func() tea.Msg {
				return ReplyMsg{RootID: rootID, Content: content}
			}
		}
		return m, nil
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// View renders the thread panel.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	borderStyle := m.styles.ThreadPanel
	if m.focused {
		borderStyle = borderStyle.BorderForeground(m.styles.ActiveBorder.GetBorderBottomForeground())
	}

	content := m.viewport.View() + "\n" + m.input.View()
	return borderStyle.Width(m.width - 2).Height(m.height - 2).Render(content)
}

func (m *Model) allPosts() []*model.Post {
	var all []*model.Post
	if m.rootPost != nil {
		all = append(all, m.rootPost)
	}
	all = append(all, m.replies...)
	return all
}

func (m *Model) ensureRenderer() {
	if m.renderer == nil {
		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithStyles(m.styles.MarkdownStyleConfig),
			glamour.WithWordWrap(max(m.width-10, 20)),
			glamour.WithPreservedNewLines(),
			glamour.WithEmoji(),
		)
	}
}

func (m *Model) updateContent() {
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
	m.viewport.SetContent(strings.Join(lines, "\n\n"))
}

func (m *Model) renderPost(p *model.Post, width int) string {
	if cached, ok := m.cache[p.ID]; ok {
		return cached
	}
	username := m.usernames[p.UserID]
	if username == "" {
		username = p.UserID[:min(8, len(p.UserID))]
	}
	pb := post.New(p, username, m.styles, width, m.renderer, m.currentUsername, 0, nil)
	rendered := pb.View()
	m.cache[p.ID] = rendered
	return rendered
}
