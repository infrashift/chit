package app

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/infrashift/chit/internal/model"
)

// mentionRe matches @username patterns at word boundaries.
// Uses the same character set as model.validUsernameRe.
var mentionRe = regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])@([a-z0-9][a-z0-9._-]{0,62}[a-z0-9])`)

// parseMentions extracts deduplicated, lowercased usernames from content.
// Also recognises @all and @channel as special keywords.
func parseMentions(content string) []string {
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	var result []string
	for _, m := range matches {
		username := strings.ToLower(m[1])
		if _, ok := seen[username]; ok {
			continue
		}
		seen[username] = struct{}{}
		result = append(result, username)
	}
	return result
}

// processMentions resolves @username mentions in a post to user IDs.
// It handles @all/@channel by collecting all channel members.
// Self-mentions and non-channel-members are filtered out.
func (a *App) processMentions(post *model.Post) ([]string, error) {
	usernames := parseMentions(post.Content)
	if len(usernames) == 0 {
		return nil, nil
	}

	// Check for @all or @channel — collect all channel members
	broadcastAll := false
	var individualUsernames []string
	for _, u := range usernames {
		if u == "all" || u == "channel" {
			broadcastAll = true
		} else {
			individualUsernames = append(individualUsernames, u)
		}
	}

	mentionedIDs := make(map[string]struct{})

	if broadcastAll {
		// Paginate through all channel members
		page := 0
		const perPage = 200
		for {
			members, err := a.Store.Channel().GetMembers(post.ChannelID, page, perPage)
			if err != nil {
				slog.Warn("processMentions: failed to get channel members", "error", err)
				break
			}
			for _, m := range members {
				mentionedIDs[m.UserID] = struct{}{}
			}
			if len(members) < perPage {
				break
			}
			page++
		}
	}

	// Resolve individual @username mentions
	for _, username := range individualUsernames {
		user, err := a.Store.User().GetByUsername(username)
		if err != nil {
			// Unknown user — skip silently
			continue
		}

		// Verify channel membership
		_, err = a.Store.Channel().GetMember(post.ChannelID, user.ID)
		if err != nil {
			// Not a channel member — skip
			continue
		}

		mentionedIDs[user.ID] = struct{}{}
	}

	// Remove self-mention
	delete(mentionedIDs, post.UserID)

	if len(mentionedIDs) == 0 {
		return nil, nil
	}

	result := make([]string, 0, len(mentionedIDs))
	for id := range mentionedIDs {
		result = append(result, id)
	}
	return result, nil
}

// notifyMentionedUsers increments mention counters and sends targeted WebSocket events.
func (a *App) notifyMentionedUsers(post *model.Post, userIDs []string) {
	for _, userID := range userIDs {
		// Increment channel mention count
		if err := a.Store.Channel().IncrementMentionCount(post.ChannelID, userID); err != nil {
			slog.Warn("notifyMentionedUsers: failed to increment channel mention count",
				"channel_id", post.ChannelID, "user_id", userID, "error", err)
		}

		// If this is a thread reply and the user follows the thread, increment thread mention count
		if post.RootID != "" {
			membership, err := a.Store.Thread().GetMembership(post.RootID, userID)
			if err == nil && membership.Following {
				if err := a.Store.Thread().IncrementMentionCount(post.RootID, userID); err != nil {
					slog.Warn("notifyMentionedUsers: failed to increment thread mention count",
						"root_id", post.RootID, "user_id", userID, "error", err)
				}
			}
		}

		// Send targeted WebSocket event
		a.Hub.Broadcast(&model.WebSocketEvent{
			Event: model.WebSocketEventMentioned,
			Data: map[string]any{
				"post_id":    post.ID,
				"channel_id": post.ChannelID,
				"user_id":    post.UserID,
			},
			Broadcast: &model.WebSocketBroadcast{
				UserID: userID,
			},
		})
	}
}
