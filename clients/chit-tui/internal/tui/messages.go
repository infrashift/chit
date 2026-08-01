package tui

import "github.com/infrashift/chit/clients/chit-tui/internal/model"

// UserLoadedMsg is sent when the current user is fetched.
type UserLoadedMsg struct {
	User *model.User
	Err  error
}

// TeamsLoadedMsg is sent when the user's teams are fetched.
type TeamsLoadedMsg struct {
	Teams []*model.Team
	Err   error
}

// ChannelsLoadedMsg is sent when a team's channels are fetched.
type ChannelsLoadedMsg struct {
	TeamID   string
	Channels []*model.Channel
	Err      error
}

// PostsLoadedMsg is sent when channel posts are fetched.
type PostsLoadedMsg struct {
	ChannelID string
	Posts     *model.PostList
	Err       error
}

// PostCreatedMsg is sent when a post is successfully created.
type PostCreatedMsg struct {
	Post *model.Post
	Err  error
}

// ThreadLoadedMsg is sent when a thread is fetched.
type ThreadLoadedMsg struct {
	PostID string
	Posts  *model.PostList
	Err    error
}

// CommandsLoadedMsg is sent when slash commands are fetched.
type CommandsLoadedMsg struct {
	Commands []*model.Command
	Err      error
}

// UsersLoadedMsg is sent when users are batch-fetched.
type UsersLoadedMsg struct {
	Users []*model.User
	Err   error
}

// WebSocketEventMsg wraps a WebSocket event for the TUI.
type WebSocketEventMsg struct {
	Event model.WebSocketEvent
}

// ChannelViewedMsg is sent when a channel view is acknowledged.
type ChannelViewedMsg struct {
	ChannelID string
	Err       error
}

// SearchResultsMsg is sent when search results are returned.
type SearchResultsMsg struct {
	Posts *model.PostList
	Err   error
}

// ChannelMembersLoadedMsg is sent when channel members are fetched for unread computation.
type ChannelMembersLoadedMsg struct {
	ChannelID string
	Members   []*model.ChannelMember
	Err       error
}

// WSConnectedMsg signals that the WebSocket connection succeeded.
type WSConnectedMsg struct{}

// WSStateMsg reports a WebSocket connect or disconnect. It is distinct from
// WSConnectedMsg, which only ever meant "the initial dial returned".
type WSStateMsg struct {
	Connected bool
	Err       error
	// Unauthorized marks a failure retrying cannot fix; the session is gone
	// and the user has to sign in again.
	Unauthorized bool
}

// DMChannelsLoadedMsg is sent when DM channels are fetched.
type DMChannelsLoadedMsg struct {
	Channels []*model.Channel
	Err      error
}

// UserSearchResultsMsg is sent when user search results are returned.
type UserSearchResultsMsg struct {
	Users []*model.User
	Err   error
}

// DMCreatedMsg is sent when a DM channel is created.
type DMCreatedMsg struct {
	Channel *model.Channel
	Err     error
}

// ChannelCreatedMsg is sent when a team channel is created.
type ChannelCreatedMsg struct {
	Channel *model.Channel
	Err     error
}

// AllTagsLoadedMsg is sent when all tags are fetched.
type AllTagsLoadedMsg struct {
	Tags []*model.Tag
	Err  error
}

// TagCreatedMsg is sent when a tag is created.
type TagCreatedMsg struct {
	Tag *model.Tag
	Err error
}

// PostTagsLoadedMsg is sent when tags for a post are fetched.
type PostTagsLoadedMsg struct {
	PostID string
	Tags   []*model.Tag
	Err    error
}

// PostsTagsLoadedMsg carries tags for many posts, keyed by post ID.
type PostsTagsLoadedMsg struct {
	Tags map[string][]*model.Tag
	Err  error
}

// TagAddedToPostMsg is sent when a tag is added to a post.
type TagAddedToPostMsg struct {
	PostID string
	TagID  string
	Err    error
}

// TagRemovedFromPostMsg is sent when a tag is removed from a post.
type TagRemovedFromPostMsg struct {
	PostID string
	TagID  string
	Err    error
}

// ChannelMemberAddedMsg is sent when a single member is added to a channel.
type ChannelMemberAddedMsg struct {
	ChannelID string
	UserID    string
	Err       error
}

// AllMembersAddedMsg is sent when all pending members have been added to a channel.
type AllMembersAddedMsg struct {
	ChannelID string
}

// ErrMsg is a generic error message.
type ErrMsg struct {
	Err error
}

// ClearErrMsg is sent after a timeout to auto-dismiss a status bar error.
type ClearErrMsg struct {
	Seq uint64
}

// PostEditedMsg reports the result of editing a post.
type PostEditedMsg struct {
	Post *model.Post
	Err  error
}

// PostDeletedMsg reports the result of deleting a post.
type PostDeletedMsg struct {
	PostID string
	Err    error
}

// OlderPostsLoadedMsg carries an older page of history, to be prepended
// rather than replacing what is displayed.
type OlderPostsLoadedMsg struct {
	ChannelID string
	Page      int
	Posts     *model.PostList
	Err       error
}
