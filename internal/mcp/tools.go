package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/model"
)

func (s *ChitMCPServer) registerTools() {
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

	// Reading Messages
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_channel_posts",
		Description: "Get posts in a channel. Use 'since' (unix ms timestamp) to poll for new messages since the last check.",
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

	// Search
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_posts",
		Description: "Search posts by text query and/or tag IDs. Provide terms, tag_ids, or both.",
	}, s.handleSearchPosts)

	// Tags
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_tags",
		Description: "List all available tags",
	}, s.handleListTags)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "add_tag_to_post",
		Description: "Add a tag to a post (message)",
	}, s.handleAddTagToPost)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_tags_for_post",
		Description: "Get all tags associated with a post",
	}, s.handleGetTagsForPost)

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
		Description: "Poll for new real-time events since the last check. Returns and clears buffered events.",
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
	PostID string `json:"post_id" jsonschema:"Thread root post ID"`
}

type followThreadArgs struct {
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
	SinceSeq int64 `json:"since_seq,omitempty" jsonschema:"Return events after this sequence number (0 for all)"`
}

// ─── Tool handlers ───────────────────────────────────────────────

func (s *ChitMCPServer) handleListTeams(_ context.Context, _ *mcp.CallToolRequest, args listTeamsArgs) (*mcp.CallToolResult, any, error) {
	perPage := args.PerPage
	if perPage == 0 {
		perPage = 60
	}
	teams, err := s.app.GetAllTeams(args.Page, perPage)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(teams)
}

func (s *ChitMCPServer) handleGetTeam(_ context.Context, _ *mcp.CallToolRequest, args getTeamArgs) (*mcp.CallToolResult, any, error) {
	team, err := s.app.GetTeam(args.TeamID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(team)
}

func (s *ChitMCPServer) handleListChannels(_ context.Context, _ *mcp.CallToolRequest, args listChannelsArgs) (*mcp.CallToolResult, any, error) {
	perPage := args.PerPage
	if perPage == 0 {
		perPage = 60
	}
	channels, err := s.app.GetChannelsForTeam(args.TeamID, args.Page, perPage)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(channels)
}

func (s *ChitMCPServer) handleGetChannel(_ context.Context, _ *mcp.CallToolRequest, args getChannelArgs) (*mcp.CallToolResult, any, error) {
	channel, err := s.app.GetChannel(args.ChannelID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(channel)
}

func (s *ChitMCPServer) handleGetChannelMembers(_ context.Context, _ *mcp.CallToolRequest, args getChannelMembersArgs) (*mcp.CallToolResult, any, error) {
	perPage := args.PerPage
	if perPage == 0 {
		perPage = 60
	}
	members, err := s.app.GetChannelMembers(args.ChannelID, args.Page, perPage)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(members)
}

func (s *ChitMCPServer) handleGetChannelPosts(ctx context.Context, _ *mcp.CallToolRequest, args getChannelPostsArgs) (*mcp.CallToolResult, any, error) {
	perPage := args.PerPage
	if perPage == 0 {
		perPage = 60
	}
	opts := model.GetPostsOptions{
		Page:    args.Page,
		PerPage: perPage,
		Since:   args.Since,
	}
	posts, err := s.app.GetPostsForChannel(ctx, args.ChannelID, opts)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(posts)
}

func (s *ChitMCPServer) handleGetPost(ctx context.Context, _ *mcp.CallToolRequest, args getPostArgs) (*mcp.CallToolResult, any, error) {
	post, err := s.app.GetPost(ctx, args.PostID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(post)
}

func (s *ChitMCPServer) handleGetPinnedPosts(ctx context.Context, _ *mcp.CallToolRequest, args getPinnedPostsArgs) (*mcp.CallToolResult, any, error) {
	posts, err := s.app.GetPinnedPosts(ctx, args.ChannelID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(posts)
}

func (s *ChitMCPServer) handleCreatePost(ctx context.Context, _ *mcp.CallToolRequest, args createPostArgs) (*mcp.CallToolResult, any, error) {
	post := &model.Post{
		ChannelID: args.ChannelID,
		UserID:    s.agentUserID,
		Content:   args.Content,
	}
	saved, err := s.app.CreatePost(ctx, post)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(saved)
}

func (s *ChitMCPServer) handleReplyToThread(ctx context.Context, _ *mcp.CallToolRequest, args replyToThreadArgs) (*mcp.CallToolResult, any, error) {
	post := &model.Post{
		ChannelID: args.ChannelID,
		UserID:    s.agentUserID,
		RootID:    args.RootID,
		Content:   args.Content,
	}
	saved, err := s.app.CreatePost(ctx, post)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(saved)
}

func (s *ChitMCPServer) handleGetThread(ctx context.Context, _ *mcp.CallToolRequest, args getThreadArgs) (*mcp.CallToolResult, any, error) {
	thread, err := s.app.GetThread(ctx, args.PostID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(thread)
}

func (s *ChitMCPServer) handleGetMyThreads(_ context.Context, _ *mcp.CallToolRequest, args getMyThreadsArgs) (*mcp.CallToolResult, any, error) {
	perPage := args.PerPage
	if perPage == 0 {
		perPage = 25
	}
	threads, err := s.app.GetThreadsForUser(s.agentUserID, args.TeamID, args.Page, perPage)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(threads)
}

func (s *ChitMCPServer) handleMarkThreadRead(_ context.Context, _ *mcp.CallToolRequest, args markThreadReadArgs) (*mcp.CallToolResult, any, error) {
	if err := s.app.MarkThreadAsRead(args.PostID, s.agentUserID); err != nil {
		return toolError(err)
	}
	return toolText("Thread marked as read")
}

func (s *ChitMCPServer) handleFollowThread(_ context.Context, _ *mcp.CallToolRequest, args followThreadArgs) (*mcp.CallToolResult, any, error) {
	if err := s.app.UpdateThreadFollowing(args.PostID, s.agentUserID, args.Following); err != nil {
		return toolError(err)
	}
	action := "followed"
	if !args.Following {
		action = "unfollowed"
	}
	return toolText(fmt.Sprintf("Thread %s", action))
}

func (s *ChitMCPServer) handleSearchPosts(ctx context.Context, _ *mcp.CallToolRequest, args searchPostsArgs) (*mcp.CallToolResult, any, error) {
	perPage := args.PerPage
	if perPage == 0 {
		perPage = 60
	}
	results, err := s.app.SearchPosts(ctx, "", args.Terms, args.TagIDs, args.Page, perPage)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(results)
}

func (s *ChitMCPServer) handleListTags(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	tags, err := s.app.GetAllTags()
	if err != nil {
		return toolError(err)
	}
	return toolJSON(tags)
}

func (s *ChitMCPServer) handleAddTagToPost(_ context.Context, _ *mcp.CallToolRequest, args addTagToPostArgs) (*mcp.CallToolResult, any, error) {
	if err := s.app.AddTagToPost(args.PostID, args.TagID); err != nil {
		return toolError(err)
	}
	return toolText("Tag added to post")
}

func (s *ChitMCPServer) handleGetTagsForPost(_ context.Context, _ *mcp.CallToolRequest, args getTagsForPostArgs) (*mcp.CallToolResult, any, error) {
	tags, err := s.app.GetTagsForPost(args.PostID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(tags)
}

func (s *ChitMCPServer) handleGetUser(_ context.Context, _ *mcp.CallToolRequest, args getUserArgs) (*mcp.CallToolResult, any, error) {
	user, err := s.app.GetUser(args.UserID)
	if err != nil {
		return toolError(err)
	}
	user.Sanitize()
	return toolJSON(user)
}

func (s *ChitMCPServer) handleGetUserByUsername(_ context.Context, _ *mcp.CallToolRequest, args getUserByUsernameArgs) (*mcp.CallToolResult, any, error) {
	user, err := s.app.GetUserByUsername(args.Username)
	if err != nil {
		return toolError(err)
	}
	user.Sanitize()
	return toolJSON(user)
}

func (s *ChitMCPServer) handleGetMyInfo(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	user, err := s.app.GetUser(s.agentUserID)
	if err != nil {
		return toolError(err)
	}
	return toolJSON(user)
}

func (s *ChitMCPServer) handleMarkChannelViewed(_ context.Context, _ *mcp.CallToolRequest, args markChannelViewedArgs) (*mcp.CallToolResult, any, error) {
	if err := s.app.UpdateChannelLastViewedAt(args.ChannelID, s.agentUserID); err != nil {
		return toolError(err)
	}
	return toolText("Channel marked as viewed")
}

func (s *ChitMCPServer) handleGetNewEvents(_ context.Context, _ *mcp.CallToolRequest, args getNewEventsArgs) (*mcp.CallToolResult, any, error) {
	events := s.eventBuffer.Drain(args.SinceSeq)
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
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}, nil, nil
}

func toolText(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}, nil, nil
}

func toolError(err error) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)},
		},
		IsError: true,
	}, nil, nil
}
