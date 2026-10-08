package bridge

import (
	"context"
	"log/slog"
	"strings"

	"github.com/infrashift/chit/internal/model"
)

// prompt builds what Claude reads for a post: the post tagged with its
// author, so that in a thread with several people Claude can tell them apart.
// With seed (a new session in a thread that already has posts, which nobody
// asked to start clean), the thread so far comes first, up to SeedMaxChars of
// it, newest kept.
func (b *Bridge) prompt(ctx context.Context, root string, post *model.Post, seed bool) string {
	msg := b.tagged(ctx, post.UserID, post.Content)
	if !seed || root == post.ID || b.cfg.SeedMaxChars <= 0 {
		return msg
	}
	history := b.threadHistory(ctx, root, post.ID)
	if history == "" {
		return msg
	}
	return "Earlier in this thread:\n\n" + history + "\n\nThe message to answer:\n\n" + msg
}

// tagged prefixes content with its author's @username, when it resolves.
func (b *Bridge) tagged(ctx context.Context, userID, content string) string {
	if user := b.author(ctx, userID); user != nil && user.Username != "" {
		return "[@" + user.Username + "] " + content
	}
	return content
}

// threadHistory renders the thread's posts before postID, one tagged post per
// paragraph, dropping the oldest to stay within SeedMaxChars. The agent's own
// replies lose their usage footer, which says nothing about the conversation.
func (b *Bridge) threadHistory(ctx context.Context, root, postID string) string {
	posts, err := b.client.GetThread(ctx, root)
	if err != nil {
		slog.Warn("failed to fetch thread to seed a new session", "root_id", root, "error", err)
		return ""
	}
	var lines []string
	for _, p := range posts {
		if p.ID == postID {
			break
		}
		content := p.Content
		if p.UserID == b.agentUserID {
			content, _, _ = strings.Cut(content, footerSeparator)
		}
		if content = strings.TrimSpace(content); content != "" {
			lines = append(lines, b.tagged(ctx, p.UserID, content))
		}
	}

	limit := b.cfg.SeedMaxChars
	size, keep := 0, len(lines)
	for keep > 0 && size+len(lines[keep-1]) <= limit {
		keep--
		size += len(lines[keep]) + 2
	}
	history := strings.Join(lines[keep:], "\n\n")
	if keep > 0 {
		history = "(earlier messages omitted)\n\n" + history
	}
	return history
}
