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
	// Tags are the names to apply to the new post once its ID is known.
	Tags []string
	// Draft is the text as written, tags included, for putting back in the
	// input when the server refuses the post.
	Draft string
	Err   error
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
	// Requested are the IDs asked for. Those missing from Users do not
	// exist as far as the server is concerned and are not asked for again.
	Requested []string
	Err       error
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

// SearchResultsMsg is sent when search results are returned. Term is the
// search they answer, so a slower earlier search can be told apart.
type SearchResultsMsg struct {
	Term  string
	Posts *model.PostList
	Err   error
}

// ChannelMembersLoadedMsg is sent when channel members are fetched for unread computation.
type ChannelMembersLoadedMsg struct {
	ChannelID string
	Members   []*model.ChannelMember
	Err       error
}

// WSConnectedMsg reports how the initial WebSocket dial went. A non-nil Err
// that is not ws.ErrUnauthorized means the dial failed but the client is
// retrying in the background; later transitions arrive as WSStateMsg.
type WSConnectedMsg struct {
	Err error
}

// WSStateMsg reports a WebSocket connect or disconnect. It is distinct from
// WSConnectedMsg, which only ever meant "the initial dial returned".
type WSStateMsg struct {
	Connected bool
	Err       error
	// Unauthorized marks a failure retrying cannot fix; the session is gone
	// and the user has to sign in again.
	Unauthorized bool
	// Desynced reports that events were dropped while the socket stayed up,
	// so the view is stale with nothing else to give it away.
	Desynced bool
}

// ProfileUpdatedMsg is sent when the signed-in user's profile is saved.
type ProfileUpdatedMsg struct {
	User *model.User
	Err  error
}

// ThreadsLoadedMsg is sent when the followed-thread lists are fetched:
// Threads in the active team, Direct in direct and group channels.
type ThreadsLoadedMsg struct {
	Threads []*model.ThreadResponse
	Direct  []*model.ThreadResponse
	Err     error
}

// ThreadFollowChangedMsg is sent when a thread's follow state is saved.
type ThreadFollowChangedMsg struct {
	RootID string
	Err    error
}

// DMChannelsLoadedMsg is sent when DM channels are fetched.
type DMChannelsLoadedMsg struct {
	Channels []*model.Channel
	Err      error
}

// MyChannelMembersLoadedMsg carries the signed-in user's member rows for a
// team's channels, from which every channel's badges are worked out.
type MyChannelMembersLoadedMsg struct {
	TeamID  string
	Members []*model.ChannelMember
	Err     error
}

// UserSearchResultsMsg is sent when user search results are returned. Term
// is the query they answer; queries fire per keystroke and race.
type UserSearchResultsMsg struct {
	Term  string
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
	// NewTag is set when the tag was created to be applied, so the known
	// tags can be extended without fetching them all again.
	NewTag *model.Tag
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

// PostPinnedMsg reports the result of pinning or unpinning a post.
type PostPinnedMsg struct {
	PostID string
	Pinned bool
	Err    error
}

// requestError reports the failure this message carries, so session
// expiry can be handled in one place rather than per message type.
func (m UserLoadedMsg) requestError() error             { return m.Err }
func (m ProfileUpdatedMsg) requestError() error         { return m.Err }
func (m ThreadsLoadedMsg) requestError() error          { return m.Err }
func (m ThreadFollowChangedMsg) requestError() error    { return m.Err }
func (m TeamsLoadedMsg) requestError() error            { return m.Err }
func (m ChannelsLoadedMsg) requestError() error         { return m.Err }
func (m PostsLoadedMsg) requestError() error            { return m.Err }
func (m PostCreatedMsg) requestError() error            { return m.Err }
func (m ThreadLoadedMsg) requestError() error           { return m.Err }
func (m CommandsLoadedMsg) requestError() error         { return m.Err }
func (m UsersLoadedMsg) requestError() error            { return m.Err }
func (m ChannelViewedMsg) requestError() error          { return m.Err }
func (m SearchResultsMsg) requestError() error          { return m.Err }
func (m MyChannelMembersLoadedMsg) requestError() error { return m.Err }
func (m ChannelMembersLoadedMsg) requestError() error   { return m.Err }
func (m WSStateMsg) requestError() error                { return m.Err }
func (m DMChannelsLoadedMsg) requestError() error       { return m.Err }
func (m UserSearchResultsMsg) requestError() error      { return m.Err }
func (m DMCreatedMsg) requestError() error              { return m.Err }
func (m ChannelCreatedMsg) requestError() error         { return m.Err }
func (m AllTagsLoadedMsg) requestError() error          { return m.Err }
func (m TagCreatedMsg) requestError() error             { return m.Err }
func (m PostTagsLoadedMsg) requestError() error         { return m.Err }
func (m PostsTagsLoadedMsg) requestError() error        { return m.Err }
func (m TagAddedToPostMsg) requestError() error         { return m.Err }
func (m TagRemovedFromPostMsg) requestError() error     { return m.Err }
func (m ChannelMemberAddedMsg) requestError() error     { return m.Err }
func (m ErrMsg) requestError() error                    { return m.Err }
func (m PostEditedMsg) requestError() error             { return m.Err }
func (m PostDeletedMsg) requestError() error            { return m.Err }
func (m OlderPostsLoadedMsg) requestError() error       { return m.Err }
func (m PostPinnedMsg) requestError() error             { return m.Err }

// ChannelLeftMsg reports the result of leaving a channel.
type ChannelLeftMsg struct {
	ChannelID string
	Err       error
}

func (m ChannelLeftMsg) requestError() error { return m.Err }
