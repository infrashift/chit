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
	File() FileStore
	Close()
}

// UserStore handles persistence for user records.
type UserStore interface {
	Save(ctx context.Context, user *model.User) (*model.User, error)
	Get(ctx context.Context, id string) (*model.User, error)
	GetByKratosID(ctx context.Context, kratosID string) (*model.User, error)
	// KratosIDDeactivated reports whether a deactivated user (delete_at set)
	// holds the Kratos identity. GetByKratosID does not see such a row, and
	// the identity cannot be provisioned again while it is there.
	KratosIDDeactivated(ctx context.Context, kratosID string) (bool, error)
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
	// Save inserts a post. With post.FileIDs it attaches those files in the
	// same transaction, and fails (409) unless each one is live, in the
	// post's channel, uploaded by the post's author and not yet attached.
	Save(ctx context.Context, post *model.Post) (*model.Post, error)
	Get(ctx context.Context, id string) (*model.Post, error)
	Update(ctx context.Context, post *model.Post) (*model.Post, error)
	// Delete soft-deletes a post. Its files are unlinked and soft-deleted
	// with it; the sweep removes their objects later.
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

// TagStore handles persistence for Tags and their applications to posts
// and channels. Reads return Tags of every owner and scope: the app filters
// them for the reader (model.Tag.VisibleTo), except GetVisible, which lists
// one reader's view.
type TagStore interface {
	// Save inserts a Tag; a name key taken in its scope is a 409.
	Save(ctx context.Context, tag *model.Tag) (*model.Tag, error)
	Get(ctx context.Context, id string) (*model.Tag, error)
	GetByIDs(ctx context.Context, ids []string) ([]*model.Tag, error)
	// GetByKey finds a Tag by name key: per owner when scope is private, per
	// channel when it is channel.
	GetByKey(ctx context.Context, scope, ownerID, channelID, nameKey string) (*model.Tag, error)
	// Update renames a Tag; a name key taken in its scope is a 409.
	Update(ctx context.Context, tag *model.Tag) (*model.Tag, error)
	// Delete removes a Tag and every application of it.
	Delete(ctx context.Context, id string) error
	// DeleteByOwner removes the owner's Tags and their applications, and
	// clears the owner as applied_by of other owners' Tags.
	DeleteByOwner(ctx context.Context, ownerID string) error
	CountByOwner(ctx context.Context, ownerID string) (int, error)
	// GetVisible lists userID's private Tags and the channel Tags of
	// channelIDs.
	GetVisible(ctx context.Context, userID string, channelIDs []string) ([]*model.Tag, error)

	AddTagToPost(ctx context.Context, messageID, tagID, appliedBy string) error
	RemoveTagFromPost(ctx context.Context, messageID, tagID string) error
	GetTagsForPost(ctx context.Context, messageID string) ([]*model.Tag, error)
	// GetTagsForPosts is the bulk form, keyed by post ID.
	GetTagsForPosts(ctx context.Context, messageIDs []string) (map[string][]*model.Tag, error)

	AddTagToChannel(ctx context.Context, channelID, tagID, appliedBy string) error
	RemoveTagFromChannel(ctx context.Context, channelID, tagID string) error
	GetTagsForChannel(ctx context.Context, channelID string) ([]*model.Tag, error)

	FilterPostIDsByTags(ctx context.Context, postIDs, tagIDs []string) ([]string, error)
	// FilterChannelIDsByTags keeps the channels that have every tag, on the
	// channel or on one of its live posts.
	FilterChannelIDsByTags(ctx context.Context, channelIDs, tagIDs []string) ([]string, error)
}

// FileStore handles persistence for attachments (model.File). Reads return
// live files only (delete_at 0), with PostID set when they are attached.
type FileStore interface {
	Save(ctx context.Context, file *model.File) (*model.File, error)
	Get(ctx context.Context, id string) (*model.File, error)
	// GetByIDs returns the live files among ids, in no particular order.
	GetByIDs(ctx context.Context, ids []string) ([]*model.File, error)
	// GetBySHA256 returns a live file in channelID with these bytes: the
	// one deduplication reuses. Never one of another channel.
	GetBySHA256(ctx context.Context, channelID, sha256 string) (*model.File, error)
	// UsageBytes is the size of the uploader's live files: their quota use.
	UsageBytes(ctx context.Context, uploaderID string) (int64, error)
	// Search finds files by name or alt text (see model.FileSearch), newest
	// first.
	Search(ctx context.Context, q *model.FileSearch) ([]*model.File, error)

	// The sweep. ExpireUnattached soft-deletes up to limit live files that
	// were created before createdBefore and are attached to no post,
	// returning how many. Deleted returns up to limit soft-deleted files
	// deleted before deletedBefore. ObjectInUse reports whether a live file
	// still uses an object. Purge removes a soft-deleted file's row.
	ExpireUnattached(ctx context.Context, createdBefore, deleteAt int64, limit int) (int, error)
	Deleted(ctx context.Context, deletedBefore int64, limit int) ([]*model.File, error)
	ObjectInUse(ctx context.Context, s3Key string) (bool, error)
	Purge(ctx context.Context, id string) error
}
