package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// FetchMe returns a command that fetches the current user.
func FetchMe(client api.ChitClient) tea.Cmd {
	return func() tea.Msg {
		u, err := client.GetMe(context.Background())
		return UserLoadedMsg{User: u, Err: err}
	}
}

// FetchTeams returns a command that fetches the user's teams.
func FetchTeams(client api.ChitClient) tea.Cmd {
	return func() tea.Msg {
		teams, err := client.GetMyTeams(context.Background())
		return TeamsLoadedMsg{Teams: teams, Err: err}
	}
}

// FetchChannels returns a command that fetches channels for a team.
func FetchChannels(client api.ChitClient, teamID string) tea.Cmd {
	return func() tea.Msg {
		channels, err := client.GetMyChannels(context.Background(), teamID)
		return ChannelsLoadedMsg{TeamID: teamID, Channels: channels, Err: err}
	}
}

// FetchPosts returns a command that fetches posts for a channel.
func FetchPosts(client api.ChitClient, channelID string, page, perPage int) tea.Cmd {
	return func() tea.Msg {
		pl, err := client.GetChannelPosts(context.Background(), channelID, page, perPage)
		return PostsLoadedMsg{ChannelID: channelID, Posts: pl, Err: err}
	}
}

// FetchOlderPosts returns a command that fetches the next page of history.
func FetchOlderPosts(client api.ChitClient, channelID string, page, perPage int) tea.Cmd {
	return func() tea.Msg {
		pl, err := client.GetChannelPosts(context.Background(), channelID, page, perPage)
		return OlderPostsLoadedMsg{ChannelID: channelID, Page: page, Posts: pl, Err: err}
	}
}

// CreatePost returns a command that creates a post. Tags ride along in the
// result, to be applied once the new post has an ID.
func CreatePost(client api.ChitClient, post *model.Post, tags ...string) tea.Cmd {
	return func() tea.Msg {
		created, err := client.CreatePost(context.Background(), post)
		return PostCreatedMsg{Post: created, Tags: tags, Err: err}
	}
}

// FetchThread returns a command that fetches a thread.
func FetchThread(client api.ChitClient, postID string) tea.Cmd {
	return func() tea.Msg {
		pl, err := client.GetThread(context.Background(), postID)
		return ThreadLoadedMsg{PostID: postID, Posts: pl, Err: err}
	}
}

// FetchCommands returns a command that fetches slash commands.
func FetchCommands(client api.ChitClient) tea.Cmd {
	return func() tea.Msg {
		cmds, err := client.GetCommands(context.Background())
		return CommandsLoadedMsg{Commands: cmds, Err: err}
	}
}

// FetchUsersByIDs returns a command that batch-fetches users.
func FetchUsersByIDs(client api.ChitClient, ids []string) tea.Cmd {
	return func() tea.Msg {
		users, err := client.GetUsersByIDs(context.Background(), ids)
		return UsersLoadedMsg{Users: users, Err: err}
	}
}

// ViewChannel returns a command that marks a channel as viewed.
func ViewChannel(client api.ChitClient, channelID string) tea.Cmd {
	return func() tea.Msg {
		err := client.ViewChannel(context.Background(), channelID)
		return ChannelViewedMsg{ChannelID: channelID, Err: err}
	}
}

// SearchPosts returns a command that searches posts in a channel.
func SearchPosts(client api.ChitClient, channelID, term string, tagIDs []string) tea.Cmd {
	return func() tea.Msg {
		pl, err := client.SearchPosts(context.Background(), channelID, term, tagIDs)
		return SearchResultsMsg{Term: term, Posts: pl, Err: err}
	}
}

// FetchChannelMembers returns a command that fetches members for a channel.
func FetchChannelMembers(client api.ChitClient, channelID string) tea.Cmd {
	return func() tea.Msg {
		members, err := client.GetChannelMembers(context.Background(), channelID)
		return ChannelMembersLoadedMsg{ChannelID: channelID, Members: members, Err: err}
	}
}

// ListenWebSocket returns a command that waits for the next WS event.
func ListenWebSocket(wsClient ws.WSClient) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-wsClient.Events()
		if !ok {
			return ErrMsg{Err: ws.ErrNotConnected}
		}
		return WebSocketEventMsg{Event: evt}
	}
}

// ListenWSState waits for the next connection-state transition. It runs
// alongside ListenWebSocket: events and transport state arrive on separate
// channels so a quiet connection is still distinguishable from a dead one.
func ListenWSState(wsClient ws.WSClient) tea.Cmd {
	return func() tea.Msg {
		st, ok := <-wsClient.State()
		if !ok {
			return nil
		}
		return WSStateMsg{
			Connected:    st.Connected,
			Err:          st.Err,
			Unauthorized: st.Unauthorized,
			Desynced:     st.Desynced,
		}
	}
}

// SearchPostsEverywhere searches every channel the user belongs to.
func SearchPostsEverywhere(client api.ChitClient, term string, tagIDs []string) tea.Cmd {
	return func() tea.Msg {
		pl, err := client.SearchPostsEverywhere(context.Background(), term, tagIDs)
		return SearchResultsMsg{Term: term, Posts: pl, Err: err}
	}
}

// FetchDMChannels returns a command that fetches the user's DM channels.
func FetchDMChannels(client api.ChitClient) tea.Cmd {
	return func() tea.Msg {
		channels, err := client.GetMyDirectChannels(context.Background())
		return DMChannelsLoadedMsg{Channels: channels, Err: err}
	}
}

// SearchUsersCmd returns a command that searches users by term.
func SearchUsersCmd(client api.ChitClient, term string) tea.Cmd {
	return func() tea.Msg {
		users, err := client.SearchUsers(context.Background(), term, 0, 20)
		return UserSearchResultsMsg{Term: term, Users: users, Err: err}
	}
}

// CreateDMChannel returns a command that creates a DM channel.
func CreateDMChannel(client api.ChitClient, userID1, userID2 string) tea.Cmd {
	return func() tea.Msg {
		ch, err := client.CreateDirectChannel(context.Background(), userID1, userID2)
		return DMCreatedMsg{Channel: ch, Err: err}
	}
}

// FetchMyThreads returns a command that loads the threads the caller follows
// in a team.
func FetchMyThreads(client api.ChitClient, teamID string) tea.Cmd {
	return func() tea.Msg {
		list, err := client.GetMyThreads(context.Background(), teamID, 0, threadInboxPageSize)
		if err != nil {
			return ThreadsLoadedMsg{Err: err}
		}
		return ThreadsLoadedMsg{Threads: list.Threads}
	}
}

// SetThreadFollowing returns a command that follows or unfollows a thread.
func SetThreadFollowing(client api.ChitClient, teamID, rootID string, following bool) tea.Cmd {
	return func() tea.Msg {
		err := client.SetThreadFollowing(context.Background(), teamID, rootID, following)
		return ThreadFollowChangedMsg{RootID: rootID, Err: err}
	}
}

// MarkThreadRead returns a command that clears a thread's unread state. The
// result is deliberately dropped: the read marker is a convenience, and a
// failure to record it should not interrupt reading the thread.
func MarkThreadRead(client api.ChitClient, teamID, rootID string) tea.Cmd {
	return func() tea.Msg {
		_ = client.MarkThreadRead(context.Background(), teamID, rootID)
		return nil
	}
}

// UpdateProfile returns a command that saves a partial profile change. Empty
// fields are left alone by the server, so only what is changing is sent.
func UpdateProfile(client api.ChitClient, patch *model.User) tea.Cmd {
	return func() tea.Msg {
		u, err := client.UpdateMe(context.Background(), patch)
		return ProfileUpdatedMsg{User: u, Err: err}
	}
}

// CreateGroupChannel returns a command that creates a group channel among the
// given users. userIDs must include the caller.
func CreateGroupChannel(client api.ChitClient, userIDs []string) tea.Cmd {
	return func() tea.Msg {
		ch, err := client.CreateGroupChannel(context.Background(), userIDs)
		return DMCreatedMsg{Channel: ch, Err: err}
	}
}

// CreateChannel returns a command that creates a team channel.
func CreateChannel(client api.ChitClient, channel *model.Channel) tea.Cmd {
	return func() tea.Msg {
		ch, err := client.CreateChannel(context.Background(), channel)
		return ChannelCreatedMsg{Channel: ch, Err: err}
	}
}

// FetchAllTags returns a command that fetches all tags.
func FetchAllTags(client api.ChitClient) tea.Cmd {
	return func() tea.Msg {
		tags, err := client.GetAllTags(context.Background())
		return AllTagsLoadedMsg{Tags: tags, Err: err}
	}
}

// FetchPostTags returns a command that fetches tags for a post.
func FetchPostTags(client api.ChitClient, postID string) tea.Cmd {
	return func() tea.Msg {
		tags, err := client.GetTagsForPost(context.Background(), postID)
		return PostTagsLoadedMsg{PostID: postID, Tags: tags, Err: err}
	}
}

// FetchPostsTags fetches tags for a whole page of history in one request.
func FetchPostsTags(client api.ChitClient, postIDs []string) tea.Cmd {
	return func() tea.Msg {
		tags, err := client.GetTagsForPosts(context.Background(), postIDs)
		return PostsTagsLoadedMsg{Tags: tags, Err: err}
	}
}

// EditPost returns a command that edits a post's text.
func EditPost(client api.ChitClient, postID, content string) tea.Cmd {
	return func() tea.Msg {
		updated, err := client.UpdatePost(context.Background(), postID, content)
		return PostEditedMsg{Post: updated, Err: err}
	}
}

// DeletePost returns a command that deletes a post.
func DeletePost(client api.ChitClient, postID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeletePost(context.Background(), postID)
		return PostDeletedMsg{PostID: postID, Err: err}
	}
}

// LeaveChannel removes the current user from a channel. The server allows
// self-removal for any member — it is leaving, not kicking.
func LeaveChannel(client api.ChitClient, channelID, userID string) tea.Cmd {
	return func() tea.Msg {
		err := client.RemoveChannelMember(context.Background(), channelID, userID)
		return ChannelLeftMsg{ChannelID: channelID, Err: err}
	}
}

// SetPostPinned pins or unpins a post. Pinning is a channel-level act, so
// any member may do it to any post — unlike editing.
func SetPostPinned(client api.ChitClient, postID string, pinned bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if pinned {
			err = client.PinPost(context.Background(), postID)
		} else {
			err = client.UnpinPost(context.Background(), postID)
		}
		return PostPinnedMsg{PostID: postID, Pinned: pinned, Err: err}
	}
}

// AddTagToPostCmd returns a command that adds a tag to a post.
func AddTagToPostCmd(client api.ChitClient, postID, tagID string) tea.Cmd {
	return func() tea.Msg {
		err := client.AddTagToPost(context.Background(), postID, tagID)
		return TagAddedToPostMsg{PostID: postID, TagID: tagID, Err: err}
	}
}

// RemoveTagFromPostCmd returns a command that removes a tag from a post.
func RemoveTagFromPostCmd(client api.ChitClient, postID, tagID string) tea.Cmd {
	return func() tea.Msg {
		err := client.RemoveTagFromPost(context.Background(), postID, tagID)
		return TagRemovedFromPostMsg{PostID: postID, TagID: tagID, Err: err}
	}
}

// AddChannelMembersCmd returns a command that adds multiple users to a channel.
func AddChannelMembersCmd(client api.ChitClient, channelID string, userIDs []string) tea.Cmd {
	return func() tea.Msg {
		for _, uid := range userIDs {
			if err := client.AddChannelMember(context.Background(), channelID, uid); err != nil {
				return ChannelMemberAddedMsg{ChannelID: channelID, UserID: uid, Err: err}
			}
		}
		return AllMembersAddedMsg{ChannelID: channelID}
	}
}

// CreateTagAndApplyCmd creates a tag then applies it to a post.
func CreateTagAndApplyCmd(client api.ChitClient, name, postID string) tea.Cmd {
	return func() tea.Msg {
		tag, err := client.CreateTag(context.Background(), name)
		if err != nil {
			return TagCreatedMsg{Tag: nil, Err: err}
		}
		applyErr := client.AddTagToPost(context.Background(), postID, tag.ID)
		if applyErr != nil {
			return TagAddedToPostMsg{PostID: postID, TagID: tag.ID, Err: applyErr}
		}
		return TagAddedToPostMsg{PostID: postID, TagID: tag.ID, Err: nil}
	}
}
