package tui_test

import (
	"context"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// mockClient implements api.ChitClient for testing.
type mockClient struct {
	me              *model.User
	teams           []*model.Team
	channels        []*model.Channel
	posts           *model.PostList
	post            *model.Post
	thread          *model.PostList
	commands        []*model.Command
	users           []*model.User
	channelMembers  []*model.ChannelMember
	dmChannels      []*model.Channel
	searchUsers     []*model.User
	dmChannel       *model.Channel
	groupChannel    *model.Channel
	createdChannel  *model.Channel
	allTags         []*model.Tag
	createdTag      *model.Tag
	postTags        []*model.Tag
	lastCreatedPost *model.Post
	channelsFetched []string // team IDs passed to GetMyChannels
	threads         []*model.ThreadResponse
	// followCalls records (rootID, following) so tests can tell an unfollow
	// that reached the server from one that only left the list.
	followCalls []followCall
	readThreads []string
	err         error
}

type followCall struct {
	RootID    string
	Following bool
}

func (m *mockClient) GetMe(_ context.Context) (*model.User, error) {
	return m.me, m.err
}

// UpdateMe applies the patch to the stored user the way the server does —
// empty fields mean "leave alone". A mock that returned a fixed user would
// pass whatever the caller sent, including nothing.
func (m *mockClient) UpdateMe(_ context.Context, patch *model.User) (*model.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	updated := *m.me
	if patch.DisplayName != "" {
		updated.DisplayName = patch.DisplayName
	}
	if patch.Username != "" {
		updated.Username = patch.Username
	}
	m.me = &updated
	return m.me, nil
}
func (m *mockClient) GetMyThreads(_ context.Context, _ string, _, _ int) (*model.UserThreadList, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.UserThreadList{Threads: m.threads, Total: int64(len(m.threads))}, nil
}

func (m *mockClient) MarkThreadRead(_ context.Context, _, threadID string) error {
	m.readThreads = append(m.readThreads, threadID)
	return m.err
}

func (m *mockClient) SetThreadFollowing(_ context.Context, _, threadID string, following bool) error {
	m.followCalls = append(m.followCalls, followCall{RootID: threadID, Following: following})
	return m.err
}

func (m *mockClient) GetMyTeams(_ context.Context) ([]*model.Team, error) {
	return m.teams, m.err
}
func (m *mockClient) GetMyChannels(_ context.Context, teamID string) ([]*model.Channel, error) {
	m.channelsFetched = append(m.channelsFetched, teamID)
	return m.channels, m.err
}
func (m *mockClient) GetChannelPosts(_ context.Context, _ string, _, _ int) (*model.PostList, error) {
	return m.posts, m.err
}
func (m *mockClient) CreatePost(_ context.Context, p *model.Post) (*model.Post, error) {
	m.lastCreatedPost = p
	if m.post != nil {
		return m.post, m.err
	}
	return p, m.err
}
func (m *mockClient) GetPost(_ context.Context, _ string) (*model.Post, error) {
	return m.post, m.err
}
func (m *mockClient) PinPost(_ context.Context, _ string) error {
	return m.err
}
func (m *mockClient) UnpinPost(_ context.Context, _ string) error {
	return m.err
}
func (m *mockClient) GetThread(_ context.Context, _ string) (*model.PostList, error) {
	return m.thread, m.err
}
func (m *mockClient) GetCommands(_ context.Context) ([]*model.Command, error) {
	return m.commands, m.err
}
func (m *mockClient) ViewChannel(_ context.Context, _ string) error {
	return m.err
}
func (m *mockClient) GetUsersByIDs(_ context.Context, _ []string) ([]*model.User, error) {
	return m.users, m.err
}
func (m *mockClient) SearchPosts(_ context.Context, _, _ string, _ []string) (*model.PostList, error) {
	return m.posts, m.err
}
func (m *mockClient) GetChannelMembers(_ context.Context, _ string) ([]*model.ChannelMember, error) {
	return m.channelMembers, m.err
}
func (m *mockClient) CreateDirectChannel(_ context.Context, _, _ string) (*model.Channel, error) {
	return m.dmChannel, m.err
}
func (m *mockClient) GetMyDirectChannels(_ context.Context) ([]*model.Channel, error) {
	return m.dmChannels, m.err
}
func (m *mockClient) SearchUsers(_ context.Context, _ string, _, _ int) ([]*model.User, error) {
	return m.searchUsers, m.err
}
func (m *mockClient) CreateGroupChannel(_ context.Context, _ []string) (*model.Channel, error) {
	return m.groupChannel, m.err
}
func (m *mockClient) CreateChannel(_ context.Context, ch *model.Channel) (*model.Channel, error) {
	if m.createdChannel != nil {
		return m.createdChannel, m.err
	}
	return ch, m.err
}
func (m *mockClient) GetAllTags(_ context.Context) ([]*model.Tag, error) {
	return m.allTags, m.err
}
func (m *mockClient) CreateTag(_ context.Context, name string) (*model.Tag, error) {
	if m.createdTag != nil {
		return m.createdTag, m.err
	}
	return &model.Tag{ID: "new-tag", Name: name}, m.err
}
func (m *mockClient) GetTagsForPost(_ context.Context, _ string) ([]*model.Tag, error) {
	return m.postTags, m.err
}
func (m *mockClient) AddTagToPost(_ context.Context, _, _ string) error {
	return m.err
}
func (m *mockClient) RemoveTagFromPost(_ context.Context, _, _ string) error {
	return m.err
}
func (m *mockClient) GetTagsForPosts(_ context.Context, ids []string) (map[string][]*model.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make(map[string][]*model.Tag, len(ids))
	for _, id := range ids {
		if len(m.postTags) > 0 {
			out[id] = m.postTags
		}
	}
	return out, nil
}

func (m *mockClient) UpdatePost(_ context.Context, postID, content string) (*model.Post, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.Post{ID: postID, Content: content, UserID: "u1", ChannelID: "c1"}, nil
}

func (m *mockClient) DeletePost(_ context.Context, _ string) error { return m.err }

func (m *mockClient) SearchPostsEverywhere(_ context.Context, _ string, _ []string) (*model.PostList, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.PostList{}, nil
}

func (m *mockClient) AddChannelMember(_ context.Context, _, _ string) error {
	return m.err
}

func (m *mockClient) RemoveChannelMember(_ context.Context, _, _ string) error {
	return m.err
}

// mockWSClient implements ws.WSClient for testing.
type mockWSClient struct {
	events chan model.WebSocketEvent
	state  chan ws.ConnState
	// closes counts Close calls. Like the real client, the channels outlive
	// Close, so listeners from one session serve the next.
	closes int
}

func newMockWSClient() *mockWSClient {
	return &mockWSClient{
		events: make(chan model.WebSocketEvent, 10),
		state:  make(chan ws.ConnState, 10),
	}
}

func (m *mockWSClient) Connect() error                      { return nil }
func (m *mockWSClient) Close() error                        { m.closes++; return nil }
func (m *mockWSClient) Events() <-chan model.WebSocketEvent { return m.events }
func (m *mockWSClient) State() <-chan ws.ConnState          { return m.state }
func (m *mockWSClient) Send(_ model.WebSocketMessage) error { return nil }
func (m *mockWSClient) SetToken(_ string)                   {}

func TestFetchMe_ReturnsUserLoadedMsg(t *testing.T) {
	client := &mockClient{me: &model.User{ID: "u1", Username: "alice"}}
	cmd := tui.FetchMe(client)
	msg := cmd()

	loaded, ok := msg.(tui.UserLoadedMsg)
	if !ok {
		t.Fatalf("expected UserLoadedMsg, got %T", msg)
	}
	if loaded.User.ID != "u1" {
		t.Errorf("user ID = %q, want %q", loaded.User.ID, "u1")
	}
	if loaded.Err != nil {
		t.Errorf("unexpected error: %v", loaded.Err)
	}
}

func TestFetchTeams_ReturnsTeamsLoadedMsg(t *testing.T) {
	client := &mockClient{teams: []*model.Team{{ID: "t1"}}}
	cmd := tui.FetchTeams(client)
	msg := cmd()

	loaded, ok := msg.(tui.TeamsLoadedMsg)
	if !ok {
		t.Fatalf("expected TeamsLoadedMsg, got %T", msg)
	}
	if len(loaded.Teams) != 1 {
		t.Errorf("teams count = %d", len(loaded.Teams))
	}
}

func TestFetchChannels_ReturnsChannelsLoadedMsg(t *testing.T) {
	client := &mockClient{channels: []*model.Channel{{ID: "c1"}}}
	cmd := tui.FetchChannels(client, "t1")
	msg := cmd()

	loaded, ok := msg.(tui.ChannelsLoadedMsg)
	if !ok {
		t.Fatalf("expected ChannelsLoadedMsg, got %T", msg)
	}
	if loaded.TeamID != "t1" {
		t.Errorf("teamID = %q", loaded.TeamID)
	}
}

func TestFetchPosts_ReturnsPostsLoadedMsg(t *testing.T) {
	client := &mockClient{posts: &model.PostList{Order: []*model.Post{{ID: "p1"}}}}
	cmd := tui.FetchPosts(client, "c1", 0, 60)
	msg := cmd()

	loaded, ok := msg.(tui.PostsLoadedMsg)
	if !ok {
		t.Fatalf("expected PostsLoadedMsg, got %T", msg)
	}
	if loaded.ChannelID != "c1" {
		t.Errorf("channelID = %q", loaded.ChannelID)
	}
}

func TestCreatePost_ReturnsPostCreatedMsg(t *testing.T) {
	created := &model.Post{ID: "new", Content: "hi"}
	client := &mockClient{post: created}
	cmd := tui.CreatePost(client, &model.Post{ChannelID: "c1", Content: "hi"})
	msg := cmd()

	m, ok := msg.(tui.PostCreatedMsg)
	if !ok {
		t.Fatalf("expected PostCreatedMsg, got %T", msg)
	}
	if m.Post.ID != "new" {
		t.Errorf("post ID = %q", m.Post.ID)
	}
}

func TestFetchThread_ReturnsThreadLoadedMsg(t *testing.T) {
	client := &mockClient{thread: &model.PostList{
		Order: []*model.Post{{ID: "p1"}},
	}}
	cmd := tui.FetchThread(client, "p1")
	msg := cmd()

	loaded, ok := msg.(tui.ThreadLoadedMsg)
	if !ok {
		t.Fatalf("expected ThreadLoadedMsg, got %T", msg)
	}
	if loaded.PostID != "p1" {
		t.Errorf("postID = %q", loaded.PostID)
	}
}

func TestFetchChannelMembers_ReturnsChannelMembersLoadedMsg(t *testing.T) {
	client := &mockClient{channelMembers: []*model.ChannelMember{
		{ChannelID: "c1", UserID: "u1", MsgCount: 5},
	}}
	cmd := tui.FetchChannelMembers(client, "c1")
	msg := cmd()

	loaded, ok := msg.(tui.ChannelMembersLoadedMsg)
	if !ok {
		t.Fatalf("expected ChannelMembersLoadedMsg, got %T", msg)
	}
	if loaded.ChannelID != "c1" {
		t.Errorf("channelID = %q", loaded.ChannelID)
	}
	if len(loaded.Members) != 1 {
		t.Errorf("members count = %d", len(loaded.Members))
	}
}

func TestFetchCommands_ReturnsCommandsLoadedMsg(t *testing.T) {
	client := &mockClient{commands: []*model.Command{{ID: "c1", Slug: "remind"}}}
	cmd := tui.FetchCommands(client)
	msg := cmd()

	loaded, ok := msg.(tui.CommandsLoadedMsg)
	if !ok {
		t.Fatalf("expected CommandsLoadedMsg, got %T", msg)
	}
	if len(loaded.Commands) != 1 {
		t.Errorf("commands count = %d", len(loaded.Commands))
	}
}

func TestFetchDMChannels_ReturnsDMChannelsLoadedMsg(t *testing.T) {
	client := &mockClient{dmChannels: []*model.Channel{{ID: "dm1", Type: "D"}}}
	cmd := tui.FetchDMChannels(client)
	msg := cmd()

	loaded, ok := msg.(tui.DMChannelsLoadedMsg)
	if !ok {
		t.Fatalf("expected DMChannelsLoadedMsg, got %T", msg)
	}
	if len(loaded.Channels) != 1 {
		t.Errorf("channels count = %d", len(loaded.Channels))
	}
	if loaded.Err != nil {
		t.Errorf("unexpected error: %v", loaded.Err)
	}
}

func TestSearchUsersCmd_ReturnsUserSearchResultsMsg(t *testing.T) {
	client := &mockClient{searchUsers: []*model.User{{ID: "u1", Username: "alice"}}}
	cmd := tui.SearchUsersCmd(client, "ali")
	msg := cmd()

	loaded, ok := msg.(tui.UserSearchResultsMsg)
	if !ok {
		t.Fatalf("expected UserSearchResultsMsg, got %T", msg)
	}
	if len(loaded.Users) != 1 {
		t.Errorf("users count = %d", len(loaded.Users))
	}
}

func TestCreateDMChannel_ReturnsDMCreatedMsg(t *testing.T) {
	client := &mockClient{dmChannel: &model.Channel{ID: "dm1", Type: "D"}}
	cmd := tui.CreateDMChannel(client, "u1", "u2")
	msg := cmd()

	loaded, ok := msg.(tui.DMCreatedMsg)
	if !ok {
		t.Fatalf("expected DMCreatedMsg, got %T", msg)
	}
	if loaded.Channel.ID != "dm1" {
		t.Errorf("channel ID = %q", loaded.Channel.ID)
	}
}

func TestCreateChannel_ReturnsChannelCreatedMsg(t *testing.T) {
	client := &mockClient{createdChannel: &model.Channel{ID: "ch1", Type: "O", Name: "deploy"}}
	cmd := tui.CreateChannel(client, &model.Channel{TeamID: "t1", Name: "deploy", Type: "O"})
	msg := cmd()

	loaded, ok := msg.(tui.ChannelCreatedMsg)
	if !ok {
		t.Fatalf("expected ChannelCreatedMsg, got %T", msg)
	}
	if loaded.Channel.ID != "ch1" {
		t.Errorf("channel ID = %q", loaded.Channel.ID)
	}
}

func TestFetchAllTags_ReturnsAllTagsLoadedMsg(t *testing.T) {
	client := &mockClient{allTags: []*model.Tag{{ID: "t1", Name: "urgent"}}}
	cmd := tui.FetchAllTags(client)
	msg := cmd()

	loaded, ok := msg.(tui.AllTagsLoadedMsg)
	if !ok {
		t.Fatalf("expected AllTagsLoadedMsg, got %T", msg)
	}
	if len(loaded.Tags) != 1 {
		t.Errorf("tags count = %d", len(loaded.Tags))
	}
}

func TestFetchPostTags_ReturnsPostTagsLoadedMsg(t *testing.T) {
	client := &mockClient{postTags: []*model.Tag{{ID: "t1", Name: "urgent"}}}
	cmd := tui.FetchPostTags(client, "p1")
	msg := cmd()

	loaded, ok := msg.(tui.PostTagsLoadedMsg)
	if !ok {
		t.Fatalf("expected PostTagsLoadedMsg, got %T", msg)
	}
	if loaded.PostID != "p1" {
		t.Errorf("postID = %q", loaded.PostID)
	}
	if len(loaded.Tags) != 1 {
		t.Errorf("tags count = %d", len(loaded.Tags))
	}
}

func TestAddTagToPostCmd_ReturnsTagAddedToPostMsg(t *testing.T) {
	client := &mockClient{}
	cmd := tui.AddTagToPostCmd(client, "p1", "t1")
	msg := cmd()

	loaded, ok := msg.(tui.TagAddedToPostMsg)
	if !ok {
		t.Fatalf("expected TagAddedToPostMsg, got %T", msg)
	}
	if loaded.PostID != "p1" || loaded.TagID != "t1" {
		t.Errorf("unexpected msg: %+v", loaded)
	}
}

func TestRemoveTagFromPostCmd_ReturnsTagRemovedFromPostMsg(t *testing.T) {
	client := &mockClient{}
	cmd := tui.RemoveTagFromPostCmd(client, "p1", "t1")
	msg := cmd()

	loaded, ok := msg.(tui.TagRemovedFromPostMsg)
	if !ok {
		t.Fatalf("expected TagRemovedFromPostMsg, got %T", msg)
	}
	if loaded.PostID != "p1" || loaded.TagID != "t1" {
		t.Errorf("unexpected msg: %+v", loaded)
	}
}

func TestAddChannelMembersCmd_ReturnsAllMembersAddedMsg(t *testing.T) {
	client := &mockClient{}
	cmd := tui.AddChannelMembersCmd(client, "c1", []string{"u2", "u3"})
	msg := cmd()

	loaded, ok := msg.(tui.AllMembersAddedMsg)
	if !ok {
		t.Fatalf("expected AllMembersAddedMsg, got %T", msg)
	}
	if loaded.ChannelID != "c1" {
		t.Errorf("channelID = %q, want c1", loaded.ChannelID)
	}
}
