package tui_test

import (
	"context"
	"errors"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/api"
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
	postsFetched    []string // channel IDs passed to GetChannelPosts
	editedPosts     []string // post IDs passed to UpdatePost
	deletedPosts    []string // post IDs passed to DeletePost
	taggedPosts     []string // "postID#tagID" passed to AddTagToPost
	membersFetched  []string // channel IDs passed to GetChannelMembers
	usersFetched    []string // user IDs passed to GetUsersByIDs
	myMembers       []*model.ChannelMember
	hang            bool // GetChannelPosts waits for its context to end
	allTagsFetches  int
	threads         []*model.ThreadResponse
	directThreads   []*model.ThreadResponse
	// teamThreadFetches records the team IDs the team inbox was fetched for.
	teamThreadFetches []string
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
func (m *mockClient) GetMyThreads(_ context.Context, teamID string, _, _ int) (*model.UserThreadList, error) {
	m.teamThreadFetches = append(m.teamThreadFetches, teamID)
	if m.err != nil {
		return nil, m.err
	}
	return &model.UserThreadList{Threads: m.threads, Total: int64(len(m.threads))}, nil
}

func (m *mockClient) GetMyDirectThreads(_ context.Context, _, _ int) (*model.UserThreadList, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.UserThreadList{Threads: m.directThreads, Total: int64(len(m.directThreads))}, nil
}

func (m *mockClient) MarkThreadRead(_ context.Context, threadID string) error {
	m.readThreads = append(m.readThreads, threadID)
	return m.err
}

func (m *mockClient) SetThreadFollowing(_ context.Context, threadID string, following bool) error {
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
func (m *mockClient) GetChannelPosts(ctx context.Context, channelID string, _, _ int) (*model.PostList, error) {
	if m.hang {
		// A server that never answers: only the context ends the wait.
		<-ctx.Done()
		return nil, ctx.Err()
	}
	m.postsFetched = append(m.postsFetched, channelID)
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
func (m *mockClient) GetUsersByIDs(_ context.Context, ids []string) ([]*model.User, error) {
	m.usersFetched = append(m.usersFetched, ids...)
	return m.users, m.err
}
func (m *mockClient) SearchPosts(_ context.Context, _, _ string, _ []string) (*model.PostList, error) {
	return m.posts, m.err
}
func (m *mockClient) GetChannelMembers(_ context.Context, channelID string) ([]*model.ChannelMember, error) {
	m.membersFetched = append(m.membersFetched, channelID)
	return m.channelMembers, m.err
}
func (m *mockClient) GetMyChannelMembers(_ context.Context, _ string) ([]*model.ChannelMember, error) {
	return m.myMembers, m.err
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
	m.allTagsFetches++
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
func (m *mockClient) AddTagToPost(_ context.Context, postID, tagID string) error {
	m.taggedPosts = append(m.taggedPosts, postID+"#"+tagID)
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
	m.editedPosts = append(m.editedPosts, postID)
	if m.err != nil {
		return nil, m.err
	}
	return &model.Post{ID: postID, Content: content, UserID: "u1", ChannelID: "c1"}, nil
}

func (m *mockClient) DeletePost(_ context.Context, postID string) error {
	m.deletedPosts = append(m.deletedPosts, postID)
	return m.err
}

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

// Fail at compile time, not with a confusing error, if the mocks fall
// behind the interfaces they stand in for.
var (
	_ api.ChitClient = (*mockClient)(nil)
	_ ws.WSClient    = (*mockWSClient)(nil)
)

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
func (m *mockWSClient) SetToken(_ string)                   {}

func TestFetchMe_ReturnsUserLoadedMsg(t *testing.T) {
	client := &mockClient{me: &model.User{ID: "u1", Username: "alice"}}
	cmd := tui.FetchMe(context.Background(), client)
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
	cmd := tui.FetchTeams(context.Background(), client)
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
	cmd := tui.FetchChannels(context.Background(), client, "t1")
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
	cmd := tui.FetchPosts(context.Background(), client, "c1", 0, 60)
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
	cmd := tui.CreatePost(context.Background(), client, &model.Post{ChannelID: "c1", Content: "hi"})
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
	cmd := tui.FetchThread(context.Background(), client, "p1")
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
	cmd := tui.FetchChannelMembers(context.Background(), client, "c1")
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
	cmd := tui.FetchCommands(context.Background(), client)
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
	cmd := tui.FetchDMChannels(context.Background(), client)
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
	cmd := tui.SearchUsersCmd(context.Background(), client, "ali")
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
	cmd := tui.CreateDMChannel(context.Background(), client, "u1", "u2")
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
	cmd := tui.CreateChannel(context.Background(), client, &model.Channel{TeamID: "t1", Name: "deploy", Type: "O"})
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
	cmd := tui.FetchAllTags(context.Background(), client)
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
	cmd := tui.FetchPostTags(context.Background(), client, "p1")
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
	cmd := tui.AddTagToPostCmd(context.Background(), client, "p1", "t1")
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
	cmd := tui.RemoveTagFromPostCmd(context.Background(), client, "p1", "t1")
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
	cmd := tui.AddChannelMembersCmd(context.Background(), client, "c1", []string{"u2", "u3"})
	msg := cmd()

	loaded, ok := msg.(tui.AllMembersAddedMsg)
	if !ok {
		t.Fatalf("expected AllMembersAddedMsg, got %T", msg)
	}
	if loaded.ChannelID != "c1" {
		t.Errorf("channelID = %q, want c1", loaded.ChannelID)
	}
}

// The inbox is two lists: the active team's threads, and those in direct and
// group channels, which belong to no team and so are never in the first.
func TestFetchMyThreads_LoadsTeamAndDirectThreads(t *testing.T) {
	client := &mockClient{
		threads:       []*model.ThreadResponse{{Thread: &model.Thread{PostID: "team"}}},
		directThreads: []*model.ThreadResponse{{Thread: &model.Thread{PostID: "dm"}}},
	}
	msg, ok := tui.FetchMyThreads(context.Background(), client, "t1")().(tui.ThreadsLoadedMsg)
	if !ok || msg.Err != nil {
		t.Fatalf("got %+v", msg)
	}
	if len(msg.Threads) != 1 || msg.Threads[0].Thread.PostID != "team" ||
		len(msg.Direct) != 1 || msg.Direct[0].Thread.PostID != "dm" {
		t.Errorf("team=%+v direct=%+v", msg.Threads, msg.Direct)
	}
}

// With no team there is no team inbox to ask for, but DM threads still are.
func TestFetchMyThreads_WithoutATeamLoadsOnlyDirectThreads(t *testing.T) {
	client := &mockClient{directThreads: []*model.ThreadResponse{{Thread: &model.Thread{PostID: "dm"}}}}
	msg := tui.FetchMyThreads(context.Background(), client, "")().(tui.ThreadsLoadedMsg)
	if len(client.teamThreadFetches) != 0 {
		t.Errorf("fetched a team inbox with no team: %v", client.teamThreadFetches)
	}
	if msg.Err != nil || len(msg.Direct) != 1 {
		t.Errorf("got %+v", msg)
	}
}

// Either list failing fails the load, so the overlay reports it rather than
// showing half an inbox as if it were all of it.
func TestFetchMyThreads_ReportsAFailure(t *testing.T) {
	for _, teamID := range []string{"t1", ""} {
		client := &mockClient{err: errors.New("boom")}
		if msg := tui.FetchMyThreads(context.Background(), client, teamID)().(tui.ThreadsLoadedMsg); msg.Err == nil {
			t.Errorf("team %q: the failure was not reported", teamID)
		}
	}
}
