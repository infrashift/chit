package post

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
	assert.That(s.ReplyIndent.GetBorderLeft(), "post.New: styles not built by styles.New")
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
	assert.That(len(badges) <= 4+len(m.tags), "Model.View: more badges than sources")
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
	assert.That(len(result) > len(header), "Model.View: header without a body line")
	if m.Post.RootID != "" {
		result = m.styles.ReplyIndent.Render(result)
	}
	return result
}

func highlightMentions(content, currentUsername string, s styles.Styles) string {
	spans := mention.FindMentions(content)
	if len(spans) == 0 {
		return content
	}

	var b strings.Builder
	last := 0
	for _, span := range spans {
		assert.That(span.Start >= last, "highlightMentions: mentions overlap or are out of order")
		assert.That(span.Start < span.End && span.End <= len(content), "highlightMentions: mention outside the content")
		assert.That(span.Username != "", "highlightMentions: mention without a name")
		b.WriteString(content[last:span.Start])
		mentionStr := content[span.Start:span.End]
		if strings.EqualFold(span.Username, currentUsername) && currentUsername != "" {
			b.WriteString(s.MentionSelfHighlight.Render(mentionStr))
		} else {
			b.WriteString(s.MentionText.Render(mentionStr))
		}
		last = span.End
	}
	assert.That(last <= len(content), "highlightMentions: consumed past the content")
	b.WriteString(content[last:])
	assert.That(b.Len() >= len(content), "highlightMentions: output lost text")
	return b.String()
}
