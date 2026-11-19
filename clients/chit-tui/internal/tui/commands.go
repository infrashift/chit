package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// FetchMe returns a command that fetches the current user.
func FetchMe(ctx context.Context, client api.ChitClient) tea.Cmd {
	assert.That(ctx != nil, "FetchMe: nil context")
	assert.That(client != nil, "FetchMe: nil client")
	return func() tea.Msg {
		u, err := client.GetMe(ctx)
		return UserLoadedMsg{User: u, Err: err}
	}
}

// FetchTeams returns a command that fetches the user's teams.
func FetchTeams(ctx context.Context, client api.ChitClient) tea.Cmd {
	assert.That(ctx != nil, "FetchTeams: nil context")
	assert.That(client != nil, "FetchTeams: nil client")
	return func() tea.Msg {
		teams, err := client.GetMyTeams(ctx)
		return TeamsLoadedMsg{Teams: teams, Err: err}
	}
}

// FetchChannels returns a command that fetches channels for a team.
func FetchChannels(ctx context.Context, client api.ChitClient, teamID string) tea.Cmd {
	assert.That(ctx != nil, "FetchChannels: nil context")
	assert.That(client != nil, "FetchChannels: nil client")
	return func() tea.Msg {
		channels, err := client.GetMyChannels(ctx, teamID)
		return ChannelsLoadedMsg{TeamID: teamID, Channels: channels, Err: err}
	}
}

// FetchPosts returns a command that fetches posts for a channel.
func FetchPosts(ctx context.Context, client api.ChitClient, channelID string, page, perPage int) tea.Cmd {
	assert.That(ctx != nil, "FetchPosts: nil context")
	assert.That(client != nil, "FetchPosts: nil client")
	assert.That(page >= 0, "FetchPosts: negative page")
	assert.That(perPage > 0, "FetchPosts: empty page size")
	return func() tea.Msg {
		pl, err := client.GetChannelPosts(ctx, channelID, page, perPage)
		return PostsLoadedMsg{ChannelID: channelID, Posts: pl, Err: err}
	}
}

// FetchOlderPosts returns a command that fetches the next page of history.
func FetchOlderPosts(ctx context.Context, client api.ChitClient, channelID string, page, perPage int) tea.Cmd {
	assert.That(ctx != nil, "FetchOlderPosts: nil context")
	assert.That(client != nil, "FetchOlderPosts: nil client")
	assert.That(page >= 1, "FetchOlderPosts: older page before page 1")
	assert.That(perPage > 0, "FetchOlderPosts: empty page size")
	return func() tea.Msg {
		pl, err := client.GetChannelPosts(ctx, channelID, page, perPage)
		return OlderPostsLoadedMsg{ChannelID: channelID, Page: page, Posts: pl, Err: err}
	}
}

// CreatePost returns a command that creates a post. Tags ride along in the
// result, to be applied once the new post has an ID.
func CreatePost(ctx context.Context, client api.ChitClient, post *model.Post, tags ...string) tea.Cmd {
	assert.That(ctx != nil, "CreatePost: nil context")
	assert.That(client != nil, "CreatePost: nil client")
	assert.That(post != nil, "CreatePost: nil post")
	return func() tea.Msg {
		created, err := client.CreatePost(ctx, post)
		msg := PostCreatedMsg{Post: created, Tags: tags, Err: err}
		if err != nil {
			msg.Draft = post.Content
			for _, t := range tags {
				msg.Draft += " #" + t
			}
		}
		return msg
	}
}

// FetchThread returns a command that fetches a thread.
func FetchThread(ctx context.Context, client api.ChitClient, postID string) tea.Cmd {
	assert.That(ctx != nil, "FetchThread: nil context")
	assert.That(client != nil, "FetchThread: nil client")
	return func() tea.Msg {
		pl, err := client.GetThread(ctx, postID)
		return ThreadLoadedMsg{PostID: postID, Posts: pl, Err: err}
	}
}

// FetchCommands returns a command that fetches slash commands.
func FetchCommands(ctx context.Context, client api.ChitClient) tea.Cmd {
	assert.That(ctx != nil, "FetchCommands: nil context")
	assert.That(client != nil, "FetchCommands: nil client")
	return func() tea.Msg {
		cmds, err := client.GetCommands(ctx)
		return CommandsLoadedMsg{Commands: cmds, Err: err}
	}
}

// FetchUsersByIDs returns a command that batch-fetches users.
func FetchUsersByIDs(ctx context.Context, client api.ChitClient, ids []string) tea.Cmd {
	assert.That(ctx != nil, "FetchUsersByIDs: nil context")
	assert.That(client != nil, "FetchUsersByIDs: nil client")
	assert.That(len(ids) > 0, "FetchUsersByIDs: no IDs to fetch")
	return func() tea.Msg {
		users, err := client.GetUsersByIDs(ctx, ids)
		return UsersLoadedMsg{Users: users, Requested: ids, Err: err}
	}
}

// ViewChannel returns a command that marks a channel as viewed.
func ViewChannel(ctx context.Context, client api.ChitClient, channelID string) tea.Cmd {
	assert.That(ctx != nil, "ViewChannel: nil context")
	assert.That(client != nil, "ViewChannel: nil client")
	return func() tea.Msg {
		err := client.ViewChannel(ctx, channelID)
		return ChannelViewedMsg{ChannelID: channelID, Err: err}
	}
}

// SearchPosts returns a command that searches posts in a channel.
func SearchPosts(ctx context.Context, client api.ChitClient, channelID, term string, tagIDs []string) tea.Cmd {
	assert.That(ctx != nil, "SearchPosts: nil context")
	assert.That(client != nil, "SearchPosts: nil client")
	return func() tea.Msg {
		pl, err := client.SearchPosts(ctx, channelID, term, tagIDs)
		return SearchResultsMsg{Term: term, Posts: pl, Err: err}
	}
}

// FetchChannelMembers returns a command that fetches members for a channel.
func FetchChannelMembers(ctx context.Context, client api.ChitClient, channelID string) tea.Cmd {
	assert.That(ctx != nil, "FetchChannelMembers: nil context")
	assert.That(client != nil, "FetchChannelMembers: nil client")
	return func() tea.Msg {
		members, err := client.GetChannelMembers(ctx, channelID)
		return ChannelMembersLoadedMsg{ChannelID: channelID, Members: members, Err: err}
	}
}

// FetchMyChannelMembers fetches the signed-in user's member rows for every
// channel in a team, in one request.
func FetchMyChannelMembers(ctx context.Context, client api.ChitClient, teamID string) tea.Cmd {
	assert.That(ctx != nil, "FetchMyChannelMembers: nil context")
	assert.That(client != nil, "FetchMyChannelMembers: nil client")
	return func() tea.Msg {
		members, err := client.GetMyChannelMembers(ctx, teamID)
		return MyChannelMembersLoadedMsg{TeamID: teamID, Members: members, Err: err}
	}
}

// ListenWebSocket returns a command that waits for the next WS event.
func ListenWebSocket(wsClient ws.WSClient) tea.Cmd {
	if wsClient == nil {
		return nil
	}
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
	if wsClient == nil {
		return nil
	}
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
func SearchPostsEverywhere(ctx context.Context, client api.ChitClient, term string, tagIDs []string) tea.Cmd {
	assert.That(ctx != nil, "SearchPostsEverywhere: nil context")
	assert.That(client != nil, "SearchPostsEverywhere: nil client")
	return func() tea.Msg {
		pl, err := client.SearchPostsEverywhere(ctx, term, tagIDs)
		return SearchResultsMsg{Term: term, Posts: pl, Err: err}
	}
}

// FetchDMChannels returns a command that fetches the user's DM channels.
func FetchDMChannels(ctx context.Context, client api.ChitClient) tea.Cmd {
	assert.That(ctx != nil, "FetchDMChannels: nil context")
	assert.That(client != nil, "FetchDMChannels: nil client")
	return func() tea.Msg {
		channels, err := client.GetMyDirectChannels(ctx)
		return DMChannelsLoadedMsg{Channels: channels, Err: err}
	}
}

// SearchUsersCmd returns a command that searches users by term.
func SearchUsersCmd(ctx context.Context, client api.ChitClient, term string) tea.Cmd {
	assert.That(ctx != nil, "SearchUsersCmd: nil context")
	assert.That(client != nil, "SearchUsersCmd: nil client")
	return func() tea.Msg {
		users, err := client.SearchUsers(ctx, term, 0, 20)
		return UserSearchResultsMsg{Term: term, Users: users, Err: err}
	}
}

// CreateDMChannel returns a command that creates a DM channel.
func CreateDMChannel(ctx context.Context, client api.ChitClient, userID1, userID2 string) tea.Cmd {
	assert.That(ctx != nil, "CreateDMChannel: nil context")
	assert.That(client != nil, "CreateDMChannel: nil client")
	return func() tea.Msg {
		ch, err := client.CreateDirectChannel(ctx, userID1, userID2)
		return DMCreatedMsg{Channel: ch, Err: err}
	}
}

// FetchMyThreads returns a command that loads the threads the caller follows:
// in teamID (skipped when empty), and in direct and group channels, which
// belong to no team and so are never in a team's list.
func FetchMyThreads(ctx context.Context, client api.ChitClient, teamID string) tea.Cmd {
	assert.That(ctx != nil, "FetchMyThreads: nil context")
	assert.That(client != nil, "FetchMyThreads: nil client")
	return func() tea.Msg {
		var msg ThreadsLoadedMsg
		if teamID != "" {
			list, err := client.GetMyThreads(ctx, teamID, 0, threadInboxPageSize)
			if err != nil {
				return ThreadsLoadedMsg{Err: err}
			}
			msg.Threads = list.Threads
		}
		direct, err := client.GetMyDirectThreads(ctx, 0, threadInboxPageSize)
		if err != nil {
			return ThreadsLoadedMsg{Err: err}
		}
		msg.Direct = direct.Threads
		return msg
	}
}

// SetThreadFollowing returns a command that follows or unfollows a thread.
func SetThreadFollowing(ctx context.Context, client api.ChitClient, rootID string, following bool) tea.Cmd {
	assert.That(ctx != nil, "SetThreadFollowing: nil context")
	assert.That(client != nil, "SetThreadFollowing: nil client")
	return func() tea.Msg {
		err := client.SetThreadFollowing(ctx, rootID, following)
		return ThreadFollowChangedMsg{RootID: rootID, Err: err}
	}
}

// MarkThreadRead returns a command that clears a thread's unread state. The
// result is only logged: the read marker is a convenience, and a failure to
// record it should not interrupt reading the thread.
func MarkThreadRead(ctx context.Context, client api.ChitClient, rootID string) tea.Cmd {
	assert.That(ctx != nil, "MarkThreadRead: nil context")
	assert.That(client != nil, "MarkThreadRead: nil client")
	return func() tea.Msg {
		errlog.Debug(client.MarkThreadRead(ctx, rootID), "mark thread read")
		return nil
	}
}

// UpdateProfile returns a command that saves a partial profile change. Empty
// fields are left alone by the server, so only what is changing is sent.
func UpdateProfile(ctx context.Context, client api.ChitClient, patch *model.User) tea.Cmd {
	assert.That(ctx != nil, "UpdateProfile: nil context")
	assert.That(client != nil, "UpdateProfile: nil client")
	assert.That(patch != nil, "UpdateProfile: nil patch")
	return func() tea.Msg {
		u, err := client.UpdateMe(ctx, patch)
		return ProfileUpdatedMsg{User: u, Err: err}
	}
}

// CreateGroupChannel returns a command that creates a group channel among the
// given users. userIDs must include the caller.
func CreateGroupChannel(ctx context.Context, client api.ChitClient, userIDs []string) tea.Cmd {
	assert.That(ctx != nil, "CreateGroupChannel: nil context")
	assert.That(client != nil, "CreateGroupChannel: nil client")
	assert.That(len(userIDs) >= minGroupChannelMembers, "CreateGroupChannel: too few members for a group")
	return func() tea.Msg {
		ch, err := client.CreateGroupChannel(ctx, userIDs)
		return DMCreatedMsg{Channel: ch, Err: err}
	}
}

// CreateChannel returns a command that creates a team channel.
func CreateChannel(ctx context.Context, client api.ChitClient, channel *model.Channel) tea.Cmd {
	assert.That(ctx != nil, "CreateChannel: nil context")
	assert.That(client != nil, "CreateChannel: nil client")
	assert.That(channel != nil, "CreateChannel: nil channel")
	return func() tea.Msg {
		ch, err := client.CreateChannel(ctx, channel)
		return ChannelCreatedMsg{Channel: ch, Err: err}
	}
}

// FetchAllTags returns a command that fetches all tags.
func FetchAllTags(ctx context.Context, client api.ChitClient) tea.Cmd {
	assert.That(ctx != nil, "FetchAllTags: nil context")
	assert.That(client != nil, "FetchAllTags: nil client")
	return func() tea.Msg {
		tags, err := client.GetAllTags(ctx)
		return AllTagsLoadedMsg{Tags: tags, Err: err}
	}
}

// FetchPostTags returns a command that fetches tags for a post.
func FetchPostTags(ctx context.Context, client api.ChitClient, postID string) tea.Cmd {
	assert.That(ctx != nil, "FetchPostTags: nil context")
	assert.That(client != nil, "FetchPostTags: nil client")
	return func() tea.Msg {
		tags, err := client.GetTagsForPost(ctx, postID)
		return PostTagsLoadedMsg{PostID: postID, Tags: tags, Err: err}
	}
}

// FetchPostsTags fetches tags for a whole page of history in one request.
func FetchPostsTags(ctx context.Context, client api.ChitClient, postIDs []string) tea.Cmd {
	assert.That(ctx != nil, "FetchPostsTags: nil context")
	assert.That(client != nil, "FetchPostsTags: nil client")
	assert.That(len(postIDs) > 0, "FetchPostsTags: no posts")
	return func() tea.Msg {
		tags, err := client.GetTagsForPosts(ctx, postIDs)
		return PostsTagsLoadedMsg{Tags: tags, Err: err}
	}
}

// EditPost returns a command that edits a post's text.
func EditPost(ctx context.Context, client api.ChitClient, postID, content string) tea.Cmd {
	assert.That(ctx != nil, "EditPost: nil context")
	assert.That(client != nil, "EditPost: nil client")
	return func() tea.Msg {
		updated, err := client.UpdatePost(ctx, postID, content)
		return PostEditedMsg{Post: updated, Err: err}
	}
}

// DeletePost returns a command that deletes a post.
func DeletePost(ctx context.Context, client api.ChitClient, postID string) tea.Cmd {
	assert.That(ctx != nil, "DeletePost: nil context")
	assert.That(client != nil, "DeletePost: nil client")
	return func() tea.Msg {
		err := client.DeletePost(ctx, postID)
		return PostDeletedMsg{PostID: postID, Err: err}
	}
}

// LeaveChannel removes the current user from a channel. The server allows
// self-removal for any member — it is leaving, not kicking.
func LeaveChannel(ctx context.Context, client api.ChitClient, channelID, userID string) tea.Cmd {
	assert.That(ctx != nil, "LeaveChannel: nil context")
	assert.That(client != nil, "LeaveChannel: nil client")
	return func() tea.Msg {
		err := client.RemoveChannelMember(ctx, channelID, userID)
		return ChannelLeftMsg{ChannelID: channelID, Err: err}
	}
}

// SetPostPinned pins or unpins a post. Pinning is a channel-level act, so
// any member may do it to any post — unlike editing.
func SetPostPinned(ctx context.Context, client api.ChitClient, postID string, pinned bool) tea.Cmd {
	assert.That(ctx != nil, "SetPostPinned: nil context")
	assert.That(client != nil, "SetPostPinned: nil client")
	return func() tea.Msg {
		var err error
		if pinned {
			err = client.PinPost(ctx, postID)
		} else {
			err = client.UnpinPost(ctx, postID)
		}
		return PostPinnedMsg{PostID: postID, Pinned: pinned, Err: err}
	}
}

// AddTagToPostCmd returns a command that adds a tag to a post.
func AddTagToPostCmd(ctx context.Context, client api.ChitClient, postID, tagID string) tea.Cmd {
	assert.That(ctx != nil, "AddTagToPostCmd: nil context")
	assert.That(client != nil, "AddTagToPostCmd: nil client")
	return func() tea.Msg {
		err := client.AddTagToPost(ctx, postID, tagID)
		return TagAddedToPostMsg{PostID: postID, TagID: tagID, Err: err}
	}
}

// RemoveTagFromPostCmd returns a command that removes a tag from a post.
func RemoveTagFromPostCmd(ctx context.Context, client api.ChitClient, postID, tagID string) tea.Cmd {
	assert.That(ctx != nil, "RemoveTagFromPostCmd: nil context")
	assert.That(client != nil, "RemoveTagFromPostCmd: nil client")
	return func() tea.Msg {
		err := client.RemoveTagFromPost(ctx, postID, tagID)
		return TagRemovedFromPostMsg{PostID: postID, TagID: tagID, Err: err}
	}
}

// AddChannelMembersCmd returns a command that adds multiple users to a channel.
func AddChannelMembersCmd(ctx context.Context, client api.ChitClient, channelID string, userIDs []string) tea.Cmd {
	assert.That(ctx != nil, "AddChannelMembersCmd: nil context")
	assert.That(client != nil, "AddChannelMembersCmd: nil client")
	assert.That(len(userIDs) > 0, "AddChannelMembersCmd: no members to add")
	return func() tea.Msg {
		for _, uid := range userIDs {
			if err := client.AddChannelMember(ctx, channelID, uid); err != nil {
				return ChannelMemberAddedMsg{ChannelID: channelID, UserID: uid, Err: err}
			}
		}
		return AllMembersAddedMsg{ChannelID: channelID}
	}
}

// CreateTagAndApplyCmd creates a tag then applies it to a post. The tag is a
// channel tag of channelID, shared with the channel's members, as tags in
// chit-tui always were; private makes it the user's private tag instead.
// With no channel it can only be private.
func CreateTagAndApplyCmd(ctx context.Context, client api.ChitClient, name, postID, channelID string, private bool) tea.Cmd {
	assert.That(ctx != nil, "CreateTagAndApplyCmd: nil context")
	assert.That(client != nil, "CreateTagAndApplyCmd: nil client")
	assert.That(name != "", "CreateTagAndApplyCmd: empty tag name")
	return func() tea.Msg {
		scope := model.TagScopeChannel
		if private || channelID == "" {
			scope, channelID = model.TagScopePrivate, ""
		}
		assert.That(scope == model.TagScopePrivate || channelID != "", "CreateTagAndApplyCmd: a channel tag without a channel")
		tag, err := client.CreateTag(ctx, name, scope, channelID)
		if err != nil {
			return TagCreatedMsg{Tag: nil, Err: err}
		}
		err = client.AddTagToPost(ctx, postID, tag.ID)
		return TagAddedToPostMsg{PostID: postID, TagID: tag.ID, NewTag: tag, Err: err}
	}
}
