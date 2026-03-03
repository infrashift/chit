package store

import "github.com/infrashift/chit/internal/model"

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
	Save(user *model.User) (*model.User, error)
	Get(id string) (*model.User, error)
	GetByKratosID(kratosID string) (*model.User, error)
	GetByUsername(username string) (*model.User, error)
	GetByEmail(email string) (*model.User, error)
	Update(user *model.User) (*model.User, error)
	Search(term string, page, perPage int) ([]*model.User, error)
	GetByIDs(ids []string) ([]*model.User, error)
}

// TeamStore handles persistence for teams and team membership.
type TeamStore interface {
	Save(team *model.Team) (*model.Team, error)
	Get(id string) (*model.Team, error)
	GetByName(name string) (*model.Team, error)
	Update(team *model.Team) (*model.Team, error)
	Delete(id string, deleteAt int64) error
	GetAll(page, perPage int) ([]*model.Team, error)
	GetTeamsForUser(userID string) ([]*model.Team, error)
	SaveMember(member *model.TeamMember) (*model.TeamMember, error)
	RemoveMember(teamID, userID string) error
	GetMembers(teamID string, page, perPage int) ([]*model.TeamMember, error)
	GetMember(teamID, userID string) (*model.TeamMember, error)
}

// ChannelStore handles persistence for channels and channel membership.
type ChannelStore interface {
	Save(channel *model.Channel) (*model.Channel, error)
	Get(id string) (*model.Channel, error)
	Update(channel *model.Channel) (*model.Channel, error)
	Delete(id string, deleteAt int64) error
	GetChannelsForTeam(teamID string, page, perPage int) ([]*model.Channel, error)
	GetChannelsForUser(userID, teamID string) ([]*model.Channel, error)
	SaveMember(member *model.ChannelMember) (*model.ChannelMember, error)
	RemoveMember(channelID, userID string) error
	GetMembers(channelID string, page, perPage int) ([]*model.ChannelMember, error)
	GetMember(channelID, userID string) (*model.ChannelMember, error)
	UpdateLastViewedAt(channelID, userID string, lastViewedAt int64) error
	GetByName(teamID, name string) (*model.Channel, error)
	SaveDirectChannel(channel *model.Channel, userIDs []string) (*model.Channel, error)
	IncrementMsgCount(channelID string, timestamp int64) error
	IncrementMentionCount(channelID, userID string) error
}

// PostStore handles persistence for posts (messages).
type PostStore interface {
	Save(post *model.Post) (*model.Post, error)
	Get(id string) (*model.Post, error)
	Update(post *model.Post) (*model.Post, error)
	Delete(id string, deleteAt int64) error
	GetPostsForChannel(channelID string, opts model.GetPostsOptions) (*model.PostList, error)
	GetPostsForThread(rootID string) (*model.PostList, error)
	GetPinnedPosts(channelID string) (*model.PostList, error)
	SetPinned(id string, pinned bool) error
}

// ThreadStore handles persistence for threads and thread memberships.
type ThreadStore interface {
	SaveOrUpdate(thread *model.Thread) error
	Get(postID string) (*model.Thread, error)
	SaveMembership(membership *model.ThreadMembership) error
	GetMembership(postID, userID string) (*model.ThreadMembership, error)
	UpdateMembership(membership *model.ThreadMembership) error
	GetThreadsForUser(userID, teamID string, page, perPage int) (*model.UserThreadList, error)
	IncrementReplyCount(postID string, timestamp int64, userID string) error
	MarkAsRead(postID, userID string, timestamp int64) error
	IncrementMentionCount(postID, userID string) error
}

// TagStore handles persistence for tags and message-tag associations.
type TagStore interface {
	Save(tag *model.Tag) (*model.Tag, error)
	GetAll() ([]*model.Tag, error)
	AddTagToPost(messageID, tagID string) error
	RemoveTagFromPost(messageID, tagID string) error
	GetTagsForPost(messageID string) ([]*model.Tag, error)
}
