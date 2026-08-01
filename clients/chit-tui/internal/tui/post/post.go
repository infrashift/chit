package post

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/mention"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// Model represents a single rendered post.
type Model struct {
	Post            *model.Post
	Username        string
	currentUsername string
	replyCount      int
	tags            []string
	styles          styles.Styles
	width           int
	renderer        *glamour.TermRenderer
}

// New creates a post model using a shared renderer.
func New(p *model.Post, username string, s styles.Styles, width int, r *glamour.TermRenderer, currentUsername string, replyCount int, tags []string) Model {
	return Model{
		Post:            p,
		Username:        username,
		currentUsername: currentUsername,
		replyCount:      replyCount,
		tags:            tags,
		styles:          s,
		width:           width,
		renderer:        r,
	}
}

// View renders the post.
func (m Model) View() string {
	if m.Post == nil {
		return ""
	}

	ts := model.MillisToTime(m.Post.CreateAt).Format("15:04")
	header := m.styles.Username.Render(m.Username) + " " + m.styles.Timestamp.Render(ts)

	var badges []string
	if m.Post.IsPinned {
		badges = append(badges, m.styles.PinBadge.Render("[pinned]"))
	}
	// EditAt has always been carried on the model but never shown, so an
	// edited message was indistinguishable from what was originally sent.
	if m.Post.EditAt > 0 {
		badges = append(badges, m.styles.Timestamp.Render("[edited]"))
	}
	if m.Post.Type == "encrypted" {
		badges = append(badges, m.styles.Timestamp.Render("[encrypted]"))
	}
	if m.replyCount > 0 {
		badges = append(badges, m.styles.Timestamp.Render(fmt.Sprintf("[%d replies]", m.replyCount)))
	}
	for _, tag := range m.tags {
		badges = append(badges, m.styles.TagBadge.Render("#"+tag))
	}
	if len(badges) > 0 {
		header += " " + strings.Join(badges, " ")
	}

	content := m.Post.Content
	if m.renderer != nil {
		rendered, err := m.renderer.Render(m.Post.Content)
		if err == nil {
			content = strings.TrimSpace(rendered)
		}
	}

	content = highlightMentions(content, m.currentUsername, m.styles)

	result := fmt.Sprintf("%s\n%s", header, content)
	if m.Post.RootID != "" {
		result = m.styles.ReplyIndent.Render(result)
	}
	return result
}

// Height returns the approximate rendered height.
func (m Model) Height() int {
	return lipgloss.Height(m.View())
}

func highlightMentions(content, currentUsername string, s styles.Styles) string {
	spans := mention.FindMentions(content)
	if len(spans) == 0 {
		return content
	}

	var b strings.Builder
	last := 0
	for _, span := range spans {
		b.WriteString(content[last:span.Start])
		mentionStr := content[span.Start:span.End]
		if strings.EqualFold(span.Username, currentUsername) && currentUsername != "" {
			b.WriteString(s.MentionSelfHighlight.Render(mentionStr))
		} else {
			b.WriteString(s.MentionText.Render(mentionStr))
		}
		last = span.End
	}
	b.WriteString(content[last:])
	return b.String()
}
