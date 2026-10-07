package store

import (
	"context"

	"github.com/infrashift/chit/internal/model"
)

// Store is the top-level interface composed of sub-store interfaces.
type Store interface {
	User() UserStore
	Team() TeamStore
	Channel() ChannelStore
	Post() PostStore
	Thread() ThreadStore
	Tag() TagStore
	Close()
}

// UserStore handles persistence for user records.
type UserStore interface {
	Save(ctx context.Context, user *model.User) (*model.User, error)
	Get(ctx context.Context, id string) (*model.User, error)
	GetByKratosID(ctx context.Context, kratosID string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByOAuthClientID(ctx context.Context, clientID string) (*model.User, error)
	Update(ctx context.Context, user *model.User) (*model.User, error)
	Search(ctx context.Context, term string, page, perPage int) ([]*model.User, error)
	GetByIDs(ctx context.Context, ids []string) ([]*model.User, error)
}

// TeamStore handles persistence for teams and team membership.
type TeamStore interface {
	Save(ctx context.Context, team *model.Team) (*model.Team, error)
	Get(ctx context.Context, id string) (*model.Team, error)
	GetByName(ctx context.Context, name string) (*model.Team, error)
	Update(ctx context.Context, team *model.Team) (*model.Team, error)
	Delete(ctx context.Context, id string, deleteAt int64) error
	// GetAll lists teams. A non-empty visibleTo limits the list to open teams
	// plus the teams that user belongs to; "" lists every team.
	GetAll(ctx context.Context, visibleTo string, page, perPage int) ([]*model.Team, error)
	GetTeamsForUser(ctx context.Context, userID string) ([]*model.Team, error)
	SaveMember(ctx context.Context, member *model.TeamMember) (*model.TeamMember, error)
	RemoveMember(ctx context.Context, teamID, userID string) error
	GetMembers(ctx context.Context, teamID string, page, perPage int) ([]*model.TeamMember, error)
	GetMember(ctx context.Context, teamID, userID string) (*model.TeamMember, error)
}

// ChannelStore handles persistence for channels and channel membership.
type ChannelStore interface {
	Save(ctx context.Context, channel *model.Channel) (*model.Channel, error)
	Get(ctx context.Context, id string) (*model.Channel, error)
	Update(ctx context.Context, channel *model.Channel) (*model.Channel, error)
	Delete(ctx context.Context, id string, deleteAt int64) error
	GetChannelsForTeam(ctx context.Context, teamID string, page, perPage int) ([]*model.Channel, error)
	GetChannelsForUser(ctx context.Context, userID, teamID string) ([]*model.Channel, error)
	SaveMember(ctx context.Context, member *model.ChannelMember) (*model.ChannelMember, error)
	RemoveMember(ctx context.Context, channelID, userID string) error
	GetMembers(ctx context.Context, channelID string, page, perPage int) ([]*model.ChannelMember, error)
	GetMember(ctx context.Context, channelID, userID string) (*model.ChannelMember, error)
	// GetMembersForUser returns userID's own member rows for the live
	// channels of teamID: the counts behind their unread and mention badges.
	GetMembersForUser(ctx context.Context, userID, teamID string) ([]*model.ChannelMember, error)
	GetChannelIDsForUser(ctx context.Context, userID string) ([]string, error)
	UpdateLastViewedAt(ctx context.Context, channelID, userID string, lastViewedAt int64) error
	GetByName(ctx context.Context, teamID, name string) (*model.Channel, error)
	SaveDirectChannel(ctx context.Context, channel *model.Channel, userIDs []string) (*model.Channel, error)
	GetDirectChannelByName(ctx context.Context, name string) (*model.Channel, error)
	GetDirectChannelsForUser(ctx context.Context, userID string) ([]*model.Channel, error)
	IncrementMsgCount(ctx context.Context, channelID string, timestamp int64) error
	// IncrementMentionCounts adds one mention for each of userIDs in channelID.
	IncrementMentionCounts(ctx context.Context, channelID string, userIDs []string) error
	// GetMemberIDs returns every member's user ID.
	GetMemberIDs(ctx context.Context, channelID string) ([]string, error)
	// GetMemberIDsByUsernames returns the user IDs of the named users who are
	// members of channelID; other names are ignored.
	GetMemberIDsByUsernames(ctx context.Context, channelID string, usernames []string) ([]string, error)
	// AddTeamMembers adds every member of the channel's team to it, returning
	// the user IDs added.
	AddTeamMembers(ctx context.Context, channelID, teamID string) ([]string, error)
	// DeleteForTeam soft-deletes every channel on a team, returning their IDs.
	DeleteForTeam(ctx context.Context, teamID string, deleteAt int64) ([]string, error)
	// RemoveMemberFromTeam removes a user from every channel on a team,
	// returning the channels they were removed from.
	RemoveMemberFromTeam(ctx context.Context, teamID, userID string) ([]string, error)
}

// PostStore handles persistence for posts (messages).
type PostStore interface {
	Save(ctx context.Context, post *model.Post) (*model.Post, error)
	Get(ctx context.Context, id string) (*model.Post, error)
	Update(ctx context.Context, post *model.Post) (*model.Post, error)
	Delete(ctx context.Context, id string, deleteAt int64) error
	GetPostsForChannel(ctx context.Context, channelID string, opts model.GetPostsOptions) (*model.PostList, error)
	GetPostsForThread(ctx context.Context, rootID string) (*model.PostList, error)
	GetPinnedPosts(ctx context.Context, channelID string) (*model.PostList, error)
	SetPinned(ctx context.Context, id string, pinned bool) error
	// GetByIDs returns the live posts among ids, in no particular order.
	GetByIDs(ctx context.Context, ids []string) ([]*model.Post, error)
	// Search returns one page of posts matching q, newest first, with every
	// filter (scope included) applied before pagination.
	Search(ctx context.Context, q *model.PostSearch) ([]*model.Post, error)
	// GetPostsSince returns posts, deleted ones included, after the cursor
	// in (update_at, id) order and updated no later than until.
	GetPostsSince(ctx context.Context, after model.PostCursor, until int64, limit int) ([]*model.Post, error)
}

// ThreadStore handles persistence for threads and thread memberships.
type ThreadStore interface {
	SaveOrUpdate(ctx context.Context, thread *model.Thread) error
	Get(ctx context.Context, postID string) (*model.Thread, error)
	SaveMembership(ctx context.Context, membership *model.ThreadMembership) error
	GetMembership(ctx context.Context, postID, userID string) (*model.ThreadMembership, error)
	UpdateMembership(ctx context.Context, membership *model.ThreadMembership) error
	GetThreadsForUser(ctx context.Context, userID, teamID string, page, perPage int) (*model.UserThreadList, error)
	// GetDirectThreadsForUser lists followed threads in direct and group
	// channels, which belong to no team.
	GetDirectThreadsForUser(ctx context.Context, userID string, page, perPage int) (*model.UserThreadList, error)
	IncrementReplyCount(ctx context.Context, postID string, timestamp int64, userID string) error
	// DecrementReplyCount accounts for a deleted reply: it lowers the count
	// and recomputes last_reply_at from the replies that remain.
	DecrementReplyCount(ctx context.Context, postID string) error
	MarkAsRead(ctx context.Context, postID, userID string, timestamp int64) error
	// IncrementMentionCounts adds one unread mention for each of userIDs that
	// follows the thread.
	IncrementMentionCounts(ctx context.Context, postID string, userIDs []string) error
}

// TagStore handles persistence for tags and message-tag associations.
type TagStore interface {
	Save(ctx context.Context, tag *model.Tag) (*model.Tag, error)
	GetAll(ctx context.Context) ([]*model.Tag, error)
	AddTagToPost(ctx context.Context, messageID, tagID string) error
	RemoveTagFromPost(ctx context.Context, messageID, tagID string) error
	GetTagsForPost(ctx context.Context, messageID string) ([]*model.Tag, error)
	// GetTagsForPosts is the bulk form, keyed by post ID.
	GetTagsForPosts(ctx context.Context, messageIDs []string) (map[string][]*model.Tag, error)
	FilterPostIDsByTags(ctx context.Context, postIDs, tagIDs []string) ([]string, error)
}
