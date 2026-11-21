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
	extra           Extra
}

// Extra is what a post shows from outside the post itself.
type Extra struct {
	// AuthorKind is the actor type of the author: "agent", "bot" or "".
	AuthorKind string
	// Via is the username of the client that posted for the author.
	Via string
	// Files holds the record of each of the post's FileIDs, in order, or
	// nil for one that has not loaded.
	Files []*model.File
}

// With returns the post with e.
func (m Model) With(e Extra) Model {
	assert.That(m.Post == nil || len(e.Files) <= len(m.Post.FileIDs), "Model.With: more file records than files")
	m.extra = e
	return m
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

// View renders the post: the header, the body, then a line for each file.
func (m Model) View() string {
	if m.Post == nil {
		return ""
	}
	lines := []string{m.header()}
	// An attachment-only post has no body line; any other post keeps one,
	// even when it is empty.
	if body := m.body(); body != "" || len(m.Post.FileIDs) == 0 {
		lines = append(lines, body)
	}
	lines = append(lines, m.fileLines()...)
	assert.That(len(lines) >= 2, "Model.View: header without a body or a file line")
	result := strings.Join(lines, "\n")
	if m.Post.RootID != "" {
		result = m.styles.ReplyIndent.Render(result)
	}
	return result
}

// header renders the author, the time and the badges.
func (m Model) header() string {
	ts := model.MillisToTime(m.Post.CreateAt).Format("15:04")
	header := m.styles.Username.Render(m.Username)
	if kind := m.extra.AuthorKind; kind == model.ActorTypeAgent || kind == model.ActorTypeBot {
		header += " " + m.styles.MentionBadge.Render("["+kind+"]")
	}
	header += " " + m.styles.Timestamp.Render(ts)
	badges := m.badges()
	if len(badges) > 0 {
		header += " " + strings.Join(badges, " ")
	}
	assert.That(strings.Contains(header, ts), "Model.header: no time in the header")
	return header
}

// badges returns the badges of the header, in a fixed order.
func (m Model) badges() []string {
	var badges []string
	if m.extra.Via != "" {
		badges = append(badges, m.styles.Timestamp.Render("[via @"+m.extra.Via+"]"))
	}
	if _, ok := m.Post.Provenance(); ok {
		badges = append(badges, m.styles.Timestamp.Render("[provenance]"))
	}
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
	assert.That(len(badges) <= 6+len(m.tags), "Model.badges: more badges than sources")
	return badges
}

// body renders the content as Markdown, with the mentions marked.
func (m Model) body() string {
	content := m.Post.Content
	if m.renderer != nil && content != "" {
		rendered, err := m.renderer.Render(content)
		if err == nil {
			content = strings.TrimSpace(rendered)
		}
	}
	out := highlightMentions(content, m.currentUsername, m.styles)
	assert.That(len(out) >= len(content), "Model.body: the mentions lost text")
	return out
}

// fileLines renders one line for each file of the post: its record when it
// has loaded, else a placeholder with the start of its ID.
func (m Model) fileLines() []string {
	lines := make([]string, 0, len(m.Post.FileIDs))
	for i, id := range m.Post.FileIDs {
		var f *model.File
		if i < len(m.extra.Files) {
			f = m.extra.Files[i]
		}
		lines = append(lines, m.styles.Timestamp.Render(FileLine(id, f)))
	}
	assert.That(len(lines) == len(m.Post.FileIDs), "Model.fileLines: a line for each file")
	return lines
}

// FileLine describes one file in one line of text: the name, the type, the
// size in pixels and in bytes, and the alt text. With no record, it names
// the start of the ID.
func FileLine(id string, f *model.File) string {
	assert.That(id != "", "FileLine: empty file ID")
	if f == nil {
		return "[file " + id[:min(8, len(id))] + "]"
	}
	parts := []string{"[file] " + f.Name, f.ContentType}
	if f.Width > 0 && f.Height > 0 {
		parts = append(parts, fmt.Sprintf("%d×%d", f.Width, f.Height))
	}
	parts = append(parts, HumanSize(f.SizeBytes))
	if f.AltText != "" {
		parts = append(parts, fmt.Sprintf("%q", f.AltText))
	}
	assert.That(len(parts) >= 3, "FileLine: parts missing")
	return strings.Join(parts, "  ")
}

// HumanSize gives a byte count as B, KB or MB, with one decimal under 10.
func HumanSize(n int64) string {
	assert.That(n >= 0, "HumanSize: negative size")
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 10<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	case n < 1<<20:
		return fmt.Sprintf("%d KB", n>>10)
	case n < 10<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d MB", n>>20)
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
