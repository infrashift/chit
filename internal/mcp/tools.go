package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/assert"
)

// registerTools registers every tool the server offers.
func (s *ChitMCPServer) registerTools() {
	assert.That(s.server != nil, "ChitMCPServer.registerTools: nil MCP server")
	s.registerDiscoveryTools()
	s.registerPostTools()
	s.registerThreadTools()
	s.registerSearchAndTagTools()
	s.registerUserAndEventTools()
	assert.That(s.client != nil, "ChitMCPServer.registerTools: tools registered without a chitd client")
}

// registerDiscoveryTools registers the team and channel discovery tools.
func (s *ChitMCPServer) registerDiscoveryTools() {
	assert.That(s.server != nil, "ChitMCPServer.registerDiscoveryTools: nil MCP server")
	// Team Discovery
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_teams",
		Description: "List all teams the agent has access to",
	}, s.handleListTeams)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_team",
		Description: "Get details of a specific team by ID",
	}, s.handleGetTeam)

	// Channel Discovery
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_channels",
		Description: "List all channels in a team",
	}, s.handleListChannels)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_channel",
		Description: "Get details of a specific channel by ID",
	}, s.handleGetChannel)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_channel_members",
		Description: "Get the list of members in a channel",
	}, s.handleGetChannelMembers)
}

// registerPostTools registers the tools that read and write messages.
func (s *ChitMCPServer) registerPostTools() {
	assert.That(s.server != nil, "ChitMCPServer.registerPostTools: nil MCP server")
	// Reading Messages
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_channel_posts",
		Description: "Get posts in a channel, newest first. Use 'since' (unix ms timestamp) to poll for messages created after that time; results are then oldest first.",
	}, s.handleGetChannelPosts)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_post",
		Description: "Get a single post by ID",
	}, s.handleGetPost)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_pinned_posts",
		Description: "Get all pinned posts in a channel",
	}, s.handleGetPinnedPosts)

	// Writing Messages
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "create_post",
		Description: "Create a new message in a channel. The agent's user_id is set automatically.",
	}, s.handleCreatePost)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "reply_to_thread",
		Description: "Reply to an existing thread. Provide the root_id of the thread and the channel_id.",
	}, s.handleReplyToThread)
}

// registerThreadTools registers the threading tools.
func (s *ChitMCPServer) registerThreadTools() {
	assert.That(s.server != nil, "ChitMCPServer.registerThreadTools: nil MCP server")
	// Threading
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_thread",
		Description: "Get all posts in a thread (root post + replies)",
	}, s.handleGetThread)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_my_threads",
		Description: "Get threads the agent is following in a team",
	}, s.handleGetMyThreads)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "mark_thread_read",
		Description: "Mark a thread as read for the agent",
	}, s.handleMarkThreadRead)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "follow_thread",
		Description: "Follow or unfollow a thread",
	}, s.handleFollowThread)
}

// registerSearchAndTagTools registers the search and tag tools.
func (s *ChitMCPServer) registerSearchAndTagTools() {
	assert.That(s.server != nil, "ChitMCPServer.registerSearchAndTagTools: nil MCP server")
	// Search
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_posts",
		Description: "Search posts by text query and/or tag IDs. Provide terms, tag_ids, or both.",
	}, s.handleSearchPosts)

	// Tags
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_tags",
		Description: "List the tags you can see: your private tags and the channel tags of your channels",
	}, s.handleListTags)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "add_tag_to_post",
		Description: "Add a tag to a post (message): a channel tag of the post's channel, or your own private tag",
	}, s.handleAddTagToPost)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_tags_for_post",
		Description: "Get the tags on a post that you can see",
	}, s.handleGetTagsForPost)
}

// registerUserAndEventTools registers the user, channel management and event tools.
func (s *ChitMCPServer) registerUserAndEventTools() {
	assert.That(s.server != nil, "ChitMCPServer.registerUserAndEventTools: nil MCP server")
	// Users
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_user",
		Description: "Get a user's profile by ID",
	}, s.handleGetUser)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_user_by_username",
		Description: "Look up a user by their username",
	}, s.handleGetUserByUsername)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_my_info",
		Description: "Get the agent's own user profile",
	}, s.handleGetMyInfo)

	// Channel Management
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "mark_channel_viewed",
		Description: "Mark a channel as viewed, clearing the unread indicator",
	}, s.handleMarkChannelViewed)

	// Events
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_new_events",
		Description: "Poll for new real-time events since the last check. Returns and clears buffered events. By default only posted, post_edited and post_deleted are returned (posts as id, channel, author, content, time); name other event types in 'types' to receive them. Events not returned are discarded.",
	}, s.handleGetNewEvents)
}

// ─── Tool argument types ─────────────────────────────────────────

type listTeamsArgs struct {
	Page    int `json:"page,omitempty"`
	PerPage int `json:"per_page,omitempty"`
}

type getTeamArgs struct {
	TeamID string `json:"team_id" jsonschema:"Team ID (UUID)"`
}

type listChannelsArgs struct {
	TeamID  string `json:"team_id" jsonschema:"Team ID"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
}

type getChannelArgs struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID (UUID)"`
}

type getChannelMembersArgs struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID"`
	Page      int    `json:"page,omitempty"`
	PerPage   int    `json:"per_page,omitempty"`
}

type getChannelPostsArgs struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID"`
	Page      int    `json:"page,omitempty"`
	PerPage   int    `json:"per_page,omitempty"`
	Since     int64  `json:"since,omitempty" jsonschema:"Unix ms timestamp; only return posts created after this time"`
}

type getPostArgs struct {
	PostID string `json:"post_id" jsonschema:"Post ID (UUID)"`
}

type getPinnedPostsArgs struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID"`
}

type createPostArgs struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID"`
	Content   string `json:"content" jsonschema:"Message content (markdown)"`
}

type replyToThreadArgs struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID"`
	RootID    string `json:"root_id" jsonschema:"Root post ID of the thread"`
	Content   string `json:"content" jsonschema:"Reply content (markdown)"`
}

type getThreadArgs struct {
	PostID string `json:"post_id" jsonschema:"Root post ID of the thread"`
}

type getMyThreadsArgs struct {
	TeamID  string `json:"team_id" jsonschema:"Team ID"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
}

type markThreadReadArgs struct {
	TeamID string `json:"team_id" jsonschema:"Team ID the thread belongs to"`
	PostID string `json:"post_id" jsonschema:"Thread root post ID"`
}

type followThreadArgs struct {
	TeamID    string `json:"team_id" jsonschema:"Team ID the thread belongs to"`
	PostID    string `json:"post_id" jsonschema:"Thread root post ID"`
	Following bool   `json:"following" jsonschema:"true to follow or false to unfollow"`
}

type searchPostsArgs struct {
	Terms   string   `json:"terms,omitempty" jsonschema:"Search query"`
	TagIDs  []string `json:"tag_ids,omitempty" jsonschema:"Tag IDs to filter by"`
	Page    int      `json:"page,omitempty"`
	PerPage int      `json:"per_page,omitempty"`
}

type addTagToPostArgs struct {
	PostID string `json:"post_id" jsonschema:"Post ID"`
	TagID  string `json:"tag_id" jsonschema:"Tag ID"`
}

type getTagsForPostArgs struct {
	PostID string `json:"post_id" jsonschema:"Post ID"`
}

type getUserArgs struct {
	UserID string `json:"user_id" jsonschema:"User ID (UUID)"`
}

type getUserByUsernameArgs struct {
	Username string `json:"username" jsonschema:"Username to look up"`
}

type markChannelViewedArgs struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID"`
}

type getNewEventsArgs struct {
	SinceSeq int64    `json:"since_seq,omitempty" jsonschema:"Return events after this sequence number (0 for all)"`
	Types    []string `json:"types,omitempty" jsonschema:"Event types to return (default: posted, post_edited, post_deleted)"`
}

// ─── Tool handlers ───────────────────────────────────────────────

// maxPerPage caps how many items one tool call or prompt may fetch.
const maxPerPage = 200

// clampPagination bounds tool-supplied pagination to sane values.
func clampPagination(page, perPage, defaultPerPage int) (clampedPage, clampedPerPage int) {
	assert.That(defaultPerPage >= 1 && defaultPerPage <= maxPerPage, "clampPagination: default per_page out of range")
	if page < 0 {
		page = 0
	}
	if perPage <= 0 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	assert.That(page >= 0, "clampPagination: negative page")
	assert.That(perPage >= 1 && perPage <= maxPerPage, "clampPagination: per_page out of range")
	return page, perPage
}

func (s *ChitMCPServer) handleListTeams(ctx context.Context, _ *mcp.CallToolRequest, args listTeamsArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleListTeams: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleListTeams: nil chitd client")
	page, perPage := clampPagination(args.Page, args.PerPage, 60)
	assert.That(page >= 0 && perPage >= 1 && perPage <= maxPerPage, "ChitMCPServer.handleListTeams: pagination not clamped")
	teams, err := s.client.GetAllTeams(ctx, page, perPage)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(teams)
}

func (s *ChitMCPServer) handleGetTeam(ctx context.Context, _ *mcp.CallToolRequest, args getTeamArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetTeam: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetTeam: nil chitd client")
	team, err := s.client.GetTeam(ctx, args.TeamID)
	if err != nil {
		return toolError(err)
	}
	assert.That(team != nil, "ChitMCPServer.handleGetTeam: nil team without an error")
	return toolJSON(team)
}

func (s *ChitMCPServer) handleListChannels(ctx context.Context, _ *mcp.CallToolRequest, args listChannelsArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleListChannels: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleListChannels: nil chitd client")
	page, perPage := clampPagination(args.Page, args.PerPage, 60)
	assert.That(page >= 0 && perPage >= 1 && perPage <= maxPerPage, "ChitMCPServer.handleListChannels: pagination not clamped")
	channels, err := s.client.GetChannelsForTeam(ctx, args.TeamID, page, perPage)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(channels)
}

func (s *ChitMCPServer) handleGetChannel(ctx context.Context, _ *mcp.CallToolRequest, args getChannelArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetChannel: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetChannel: nil chitd client")
	channel, err := s.client.GetChannel(ctx, args.ChannelID)
	if err != nil {
		return toolError(err)
	}
	assert.That(channel != nil, "ChitMCPServer.handleGetChannel: nil channel without an error")
	return toolJSON(channel)
}

func (s *ChitMCPServer) handleGetChannelMembers(ctx context.Context, _ *mcp.CallToolRequest, args getChannelMembersArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetChannelMembers: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetChannelMembers: nil chitd client")
	page, perPage := clampPagination(args.Page, args.PerPage, 60)
	assert.That(page >= 0 && perPage >= 1 && perPage <= maxPerPage, "ChitMCPServer.handleGetChannelMembers: pagination not clamped")
	members, err := s.client.GetChannelMembers(ctx, args.ChannelID, page, perPage)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(members)
}

func (s *ChitMCPServer) handleGetChannelPosts(ctx context.Context, _ *mcp.CallToolRequest, args getChannelPostsArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetChannelPosts: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetChannelPosts: nil chitd client")
	page, perPage := clampPagination(args.Page, args.PerPage, 60)
	assert.That(page >= 0 && perPage >= 1 && perPage <= maxPerPage, "ChitMCPServer.handleGetChannelPosts: pagination not clamped")
	posts, err := s.client.GetChannelPosts(ctx, args.ChannelID, page, perPage, args.Since)
	if err != nil {
		return toolError(err)
	}
	assert.That(posts != nil, "ChitMCPServer.handleGetChannelPosts: nil post list without an error")
	return toolJSON(viewPostList(posts))
}

func (s *ChitMCPServer) handleGetPost(ctx context.Context, _ *mcp.CallToolRequest, args getPostArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetPost: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetPost: nil chitd client")
	post, err := s.client.GetPost(ctx, args.PostID)
	if err != nil {
		return toolError(err)
	}
	assert.That(post != nil, "ChitMCPServer.handleGetPost: nil post without an error")
	return toolJSON(viewPost(post))
}

func (s *ChitMCPServer) handleGetPinnedPosts(ctx context.Context, _ *mcp.CallToolRequest, args getPinnedPostsArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetPinnedPosts: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetPinnedPosts: nil chitd client")
	posts, err := s.client.GetPinnedPosts(ctx, args.ChannelID)
	if err != nil {
		return toolError(err)
	}
	assert.That(posts != nil, "ChitMCPServer.handleGetPinnedPosts: nil post list without an error")
	return toolJSON(viewPostList(posts))
}

func (s *ChitMCPServer) handleCreatePost(ctx context.Context, _ *mcp.CallToolRequest, args createPostArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleCreatePost: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleCreatePost: nil chitd client")
	saved, err := s.client.CreatePost(ctx, args.ChannelID, "", args.Content, nil)
	if err != nil {
		return toolError(err)
	}
	assert.That(saved != nil, "ChitMCPServer.handleCreatePost: nil saved without an error")
	return toolJSON(viewPost(saved))
}

func (s *ChitMCPServer) handleReplyToThread(ctx context.Context, _ *mcp.CallToolRequest, args replyToThreadArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleReplyToThread: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleReplyToThread: nil chitd client")
	saved, err := s.client.CreatePost(ctx, args.ChannelID, args.RootID, args.Content, nil)
	if err != nil {
		return toolError(err)
	}
	assert.That(saved != nil, "ChitMCPServer.handleReplyToThread: nil saved without an error")
	return toolJSON(viewPost(saved))
}

func (s *ChitMCPServer) handleGetThread(ctx context.Context, _ *mcp.CallToolRequest, args getThreadArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetThread: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetThread: nil chitd client")
	posts, err := s.client.GetThread(ctx, args.PostID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(viewPosts(posts))
}

func (s *ChitMCPServer) handleGetMyThreads(ctx context.Context, _ *mcp.CallToolRequest, args getMyThreadsArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetMyThreads: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetMyThreads: nil chitd client")
	page, perPage := clampPagination(args.Page, args.PerPage, 25)
	assert.That(page >= 0 && perPage >= 1 && perPage <= maxPerPage, "ChitMCPServer.handleGetMyThreads: pagination not clamped")
	threads, err := s.client.GetMyThreads(ctx, args.TeamID, page, perPage)
	if err != nil {
		return toolError(err)
	}
	assert.That(threads != nil, "ChitMCPServer.handleGetMyThreads: nil threads without an error")
	return toolJSON(threads)
}

func (s *ChitMCPServer) handleMarkThreadRead(ctx context.Context, _ *mcp.CallToolRequest, args markThreadReadArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleMarkThreadRead: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleMarkThreadRead: nil chitd client")
	if err := s.client.MarkThreadAsRead(ctx, args.TeamID, args.PostID); err != nil {
		return toolError(err)
	}
	return toolText("Thread marked as read")
}

func (s *ChitMCPServer) handleFollowThread(ctx context.Context, _ *mcp.CallToolRequest, args followThreadArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleFollowThread: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleFollowThread: nil chitd client")
	if err := s.client.UpdateThreadFollowing(ctx, args.TeamID, args.PostID, args.Following); err != nil {
		return toolError(err)
	}
	action := "followed"
	if !args.Following {
		action = "unfollowed"
	}
	return toolText(fmt.Sprintf("Thread %s", action))
}

func (s *ChitMCPServer) handleSearchPosts(ctx context.Context, _ *mcp.CallToolRequest, args searchPostsArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleSearchPosts: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleSearchPosts: nil chitd client")
	page, perPage := clampPagination(args.Page, args.PerPage, 60)
	assert.That(page >= 0 && perPage >= 1 && perPage <= maxPerPage, "ChitMCPServer.handleSearchPosts: pagination not clamped")
	results, err := s.client.SearchPosts(ctx, args.Terms, args.TagIDs, page, perPage)
	if err != nil {
		return toolError(err)
	}
	assert.That(results != nil, "ChitMCPServer.handleSearchPosts: nil post list without an error")
	return toolJSON(viewPostList(results))
}

func (s *ChitMCPServer) handleListTags(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleListTags: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleListTags: nil chitd client")
	tags, err := s.client.GetAllTags(ctx)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(tags)
}

func (s *ChitMCPServer) handleAddTagToPost(ctx context.Context, _ *mcp.CallToolRequest, args addTagToPostArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleAddTagToPost: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleAddTagToPost: nil chitd client")
	if err := s.client.AddTagToPost(ctx, args.PostID, args.TagID); err != nil {
		return toolError(err)
	}
	return toolText("Tag added to post")
}

func (s *ChitMCPServer) handleGetTagsForPost(ctx context.Context, _ *mcp.CallToolRequest, args getTagsForPostArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetTagsForPost: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetTagsForPost: nil chitd client")
	tags, err := s.client.GetTagsForPost(ctx, args.PostID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(tags)
}

func (s *ChitMCPServer) handleGetUser(ctx context.Context, _ *mcp.CallToolRequest, args getUserArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetUser: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetUser: nil chitd client")
	user, err := s.client.GetUser(ctx, args.UserID)
	if err != nil {
		return toolError(err)
	}
	assert.That(user != nil, "ChitMCPServer.handleGetUser: nil user without an error")
	return toolJSON(user)
}

func (s *ChitMCPServer) handleGetUserByUsername(ctx context.Context, _ *mcp.CallToolRequest, args getUserByUsernameArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetUserByUsername: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetUserByUsername: nil chitd client")
	user, err := s.client.GetUserByUsername(ctx, args.Username)
	if err != nil {
		return toolError(err)
	}
	assert.That(user != nil, "ChitMCPServer.handleGetUserByUsername: nil user without an error")
	return toolJSON(user)
}

func (s *ChitMCPServer) handleGetMyInfo(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleGetMyInfo: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleGetMyInfo: nil chitd client")
	user, err := s.client.Me(ctx)
	if err != nil {
		return toolError(err)
	}
	assert.That(user != nil, "ChitMCPServer.handleGetMyInfo: nil user without an error")
	return toolJSON(user)
}

func (s *ChitMCPServer) handleMarkChannelViewed(ctx context.Context, _ *mcp.CallToolRequest, args markChannelViewedArgs) (*mcp.CallToolResult, any, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleMarkChannelViewed: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleMarkChannelViewed: nil chitd client")
	if err := s.client.ViewChannel(ctx, args.ChannelID); err != nil {
		return toolError(err)
	}
	return toolText("Channel marked as viewed")
}

func (s *ChitMCPServer) handleGetNewEvents(ctx context.Context, _ *mcp.CallToolRequest, args getNewEventsArgs) (*mcp.CallToolResult, any, error) {
	assert.That(s.eventBuffer != nil, "ChitMCPServer.handleGetNewEvents: nil event buffer")
	events := viewEvents(s.eventBuffer.Drain(args.SinceSeq), args.Types)
	assert.That(len(events) <= maxEventBufferSize, "ChitMCPServer.handleGetNewEvents: more events than the buffer holds")
	if len(events) == 0 {
		return toolText("No new events")
	}
	return toolJSON(events)
}

// ─── Helpers ─────────────────────────────────────────────────────

func toolJSON(data any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return toolError(err)
	}
	assert.That(len(b) > 0, "toolJSON: empty encoding without an error")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}, nil, nil
}

func toolText(msg string) (*mcp.CallToolResult, any, error) {
	assert.That(msg != "", "toolText: empty message")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}, nil, nil
}

func toolError(err error) (*mcp.CallToolResult, any, error) {
	assert.That(err != nil, "toolError: nil error")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
		},
		IsError: true,
	}, nil, nil
}
