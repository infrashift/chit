package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/model"
)

// ─── Helper ──────────────────────────────────────────────────────

func callTool(t *testing.T, ctx context.Context, cs *mcpsdk.ClientSession, name string, args any) *mcpsdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

func extractText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("no content in tool result")
	}
	tc, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

// ─── Tool tests ──────────────────────────────────────────────────

func TestMCP_ListTools(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 23 {
		t.Fatalf("expected 23 tools, got %d", len(res.Tools))
	}

	expected := map[string]bool{
		"list_teams": false, "get_team": false, "list_channels": false,
		"get_channel": false, "get_channel_members": false, "get_channel_posts": false,
		"get_post": false, "get_pinned_posts": false, "create_post": false,
		"reply_to_thread": false, "get_thread": false, "get_my_threads": false,
		"mark_thread_read": false, "follow_thread": false, "search_posts": false,
		"list_tags": false, "add_tag_to_post": false, "get_tags_for_post": false,
		"get_user": false, "get_user_by_username": false, "get_my_info": false,
		"mark_channel_viewed": false, "get_new_events": false,
	}
	for _, tool := range res.Tools {
		if _, ok := expected[tool.Name]; ok {
			expected[tool.Name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("tool %q not found", name)
		}
	}
}

func TestMCP_ListTeams(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "list_teams", map[string]any{})
	text := extractText(t, res)

	var teams []*model.Team
	if err := json.Unmarshal([]byte(text), &teams); err != nil {
		t.Fatalf("unmarshal teams: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
	if teams[0].Name != "engineering" {
		t.Fatalf("expected team name 'engineering', got %q", teams[0].Name)
	}
}

func TestMCP_GetTeam(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_team", map[string]any{"team_id": teamID})
	text := extractText(t, res)

	var team model.Team
	if err := json.Unmarshal([]byte(text), &team); err != nil {
		t.Fatalf("unmarshal team: %v", err)
	}
	if team.ID != teamID || team.Name != "engineering" {
		t.Fatalf("unexpected team: %+v", team)
	}
}

func TestMCP_ListChannels(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "list_channels", map[string]any{"team_id": teamID})
	text := extractText(t, res)

	var channels []*model.Channel
	if err := json.Unmarshal([]byte(text), &channels); err != nil {
		t.Fatalf("unmarshal channels: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Name != "general" {
		t.Fatalf("expected channel name 'general', got %q", channels[0].Name)
	}
}

func TestMCP_GetChannel(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_channel", map[string]any{"channel_id": channelID})
	text := extractText(t, res)

	var ch model.Channel
	if err := json.Unmarshal([]byte(text), &ch); err != nil {
		t.Fatalf("unmarshal channel: %v", err)
	}
	if ch.ID != channelID || ch.Name != "general" {
		t.Fatalf("unexpected channel: %+v", ch)
	}
}

func TestMCP_GetChannelMembers(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_channel_members", map[string]any{"channel_id": channelID})
	text := extractText(t, res)

	var members []*model.ChannelMember
	if err := json.Unmarshal([]byte(text), &members); err != nil {
		t.Fatalf("unmarshal members: %v", err)
	}
	if len(members) < 1 {
		t.Fatal("expected at least 1 channel member")
	}
}

func TestMCP_GetChannelPosts(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_channel_posts", map[string]any{"channel_id": channelID})
	text := extractText(t, res)

	var postList model.PostList
	if err := json.Unmarshal([]byte(text), &postList); err != nil {
		t.Fatalf("unmarshal post list: %v", err)
	}
	if len(postList.Order) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(postList.Order))
	}
}

func TestMCP_GetPost(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_post", map[string]any{"post_id": rootPostID})
	text := extractText(t, res)

	var post model.Post
	if err := json.Unmarshal([]byte(text), &post); err != nil {
		t.Fatalf("unmarshal post: %v", err)
	}
	if post.ID != rootPostID || post.Content != "Hello world" {
		t.Fatalf("unexpected post: %+v", post)
	}
}

func TestMCP_CreatePost(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "create_post", map[string]any{
		"channel_id": channelID,
		"content":    "Test message from agent",
	})
	text := extractText(t, res)

	var post model.Post
	if err := json.Unmarshal([]byte(text), &post); err != nil {
		t.Fatalf("unmarshal post: %v", err)
	}
	if post.UserID != agentUserID {
		t.Fatalf("expected agent user_id %q, got %q", agentUserID, post.UserID)
	}
	if post.Content != "Test message from agent" {
		t.Fatalf("unexpected content: %q", post.Content)
	}
	if post.ChannelID != channelID {
		t.Fatalf("unexpected channel_id: %q", post.ChannelID)
	}
}

func TestMCP_ReplyToThread(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "reply_to_thread", map[string]any{
		"channel_id": channelID,
		"root_id":    rootPostID,
		"content":    "Thread reply",
	})
	text := extractText(t, res)

	var post model.Post
	if err := json.Unmarshal([]byte(text), &post); err != nil {
		t.Fatalf("unmarshal post: %v", err)
	}
	if post.RootID != rootPostID {
		t.Fatalf("expected root_id %q, got %q", rootPostID, post.RootID)
	}
	if post.UserID != agentUserID {
		t.Fatalf("expected user_id %q, got %q", agentUserID, post.UserID)
	}
}

func TestMCP_GetThread(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_thread", map[string]any{"post_id": rootPostID})
	text := extractText(t, res)

	var postList model.PostList
	if err := json.Unmarshal([]byte(text), &postList); err != nil {
		t.Fatalf("unmarshal thread: %v", err)
	}
	if len(postList.Order) < 2 {
		t.Fatalf("expected at least 2 posts in thread, got %d", len(postList.Order))
	}
}

func TestMCP_GetMyThreads(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_my_threads", map[string]any{"team_id": teamID})
	text := extractText(t, res)

	var threadList model.UserThreadList
	if err := json.Unmarshal([]byte(text), &threadList); err != nil {
		t.Fatalf("unmarshal thread list: %v", err)
	}
	// The mock returns an empty list; just verify the call succeeds.
}

func TestMCP_MarkThreadRead(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "mark_thread_read", map[string]any{"team_id": teamID, "post_id": rootPostID})
	text := extractText(t, res)
	if text != "Thread marked as read" {
		t.Fatalf("unexpected response: %q", text)
	}
}

func TestMCP_FollowThread(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	// Follow
	res := callTool(t, ctx, cs, "follow_thread", map[string]any{
		"team_id":   teamID,
		"post_id":   rootPostID,
		"following": true,
	})
	text := extractText(t, res)
	if text != "Thread followed" {
		t.Fatalf("expected 'Thread followed', got %q", text)
	}

	// Unfollow
	res = callTool(t, ctx, cs, "follow_thread", map[string]any{
		"team_id":   teamID,
		"post_id":   rootPostID,
		"following": false,
	})
	text = extractText(t, res)
	if text != "Thread unfollowed" {
		t.Fatalf("expected 'Thread unfollowed', got %q", text)
	}
}

func TestMCP_SearchPosts(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	// SQL fallback: ZincSearchURL is empty in test config, so SearchByContent is used
	res := callTool(t, ctx, cs, "search_posts", map[string]any{"terms": "Hello"})
	if res.IsError {
		t.Fatalf("expected success from search_posts, got error: %s", extractText(t, res))
	}
	text := extractText(t, res)

	var postList model.PostList
	if err := json.Unmarshal([]byte(text), &postList); err != nil {
		t.Fatalf("unmarshal post list: %v", err)
	}
	if len(postList.Order) != 1 {
		t.Fatalf("expected 1 post matching 'Hello', got %d", len(postList.Order))
	}
	if postList.Order[0].ID != rootPostID {
		t.Fatalf("expected post %s, got %s", rootPostID, postList.Order[0].ID)
	}
}

func TestMCP_ListTags(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "list_tags", map[string]any{})
	text := extractText(t, res)

	var tags []*model.Tag
	if err := json.Unmarshal([]byte(text), &tags); err != nil {
		t.Fatalf("unmarshal tags: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "important" {
		t.Fatalf("expected 1 tag 'important', got %+v", tags)
	}
}

func TestMCP_AddTagToPost(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "add_tag_to_post", map[string]any{
		"post_id": replyPostID,
		"tag_id":  tagID,
	})
	text := extractText(t, res)
	if text != "Tag added to post" {
		t.Fatalf("unexpected response: %q", text)
	}
}

func TestMCP_GetTagsForPost(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_tags_for_post", map[string]any{"post_id": rootPostID})
	text := extractText(t, res)

	var tags []*model.Tag
	if err := json.Unmarshal([]byte(text), &tags); err != nil {
		t.Fatalf("unmarshal tags: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag for post, got %d", len(tags))
	}
}

func TestMCP_GetUser(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_user", map[string]any{"user_id": extraUserID})
	text := extractText(t, res)

	var user model.User
	if err := json.Unmarshal([]byte(text), &user); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	if user.ID != extraUserID {
		t.Fatalf("unexpected user id: %q", user.ID)
	}
	// Email should be redacted via Sanitize()
	if user.Email != "" {
		t.Fatalf("expected email to be sanitized (empty), got %q", user.Email)
	}
}

func TestMCP_GetUserByUsername(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_user_by_username", map[string]any{"username": "alice"})
	text := extractText(t, res)

	var user model.User
	if err := json.Unmarshal([]byte(text), &user); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("expected username 'alice', got %q", user.Username)
	}
	if user.Email != "" {
		t.Fatalf("expected sanitized email, got %q", user.Email)
	}
}

func TestMCP_GetMyInfo(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "get_my_info", map[string]any{})
	text := extractText(t, res)

	var user model.User
	if err := json.Unmarshal([]byte(text), &user); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	if user.ID != agentUserID {
		t.Fatalf("expected agent user id, got %q", user.ID)
	}
	// GetMyInfo does NOT sanitize — email should be present
	if user.Email == "" {
		t.Fatal("expected email to be present for own profile")
	}
}

func TestMCP_MarkChannelViewed(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res := callTool(t, ctx, cs, "mark_channel_viewed", map[string]any{"channel_id": channelID})
	text := extractText(t, res)
	if text != "Channel marked as viewed" {
		t.Fatalf("unexpected response: %q", text)
	}
}

func TestMCP_GetNewEvents(t *testing.T) {
	ctx, cs, srv, cleanup := setupTestMCP(t)
	defer cleanup()

	// Push an event into the buffer
	srv.eventBuffer.Push(&model.WebSocketEvent{
		Event: model.WebSocketEventPosted,
		Data:  map[string]any{"msg": "test"},
	})

	res := callTool(t, ctx, cs, "get_new_events", map[string]any{"since_seq": 0})
	text := extractText(t, res)

	var events []*model.WebSocketEvent
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		t.Fatalf("unmarshal events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != model.WebSocketEventPosted {
		t.Fatalf("expected 'posted' event, got %q", events[0].Event)
	}

	// Second poll should be empty
	res2 := callTool(t, ctx, cs, "get_new_events", map[string]any{"since_seq": 0})
	text2 := extractText(t, res2)
	if text2 != "No new events" {
		t.Fatalf("expected 'No new events', got %q", text2)
	}
}

// ─── Resource tests ──────────────────────────────────────────────

func TestMCP_Resource_AgentIdentity(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res, err := cs.ReadResource(ctx, &mcpsdk.ReadResourceParams{
		URI: "chit://agent/identity",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) == 0 {
		t.Fatal("expected resource contents")
	}

	var user model.User
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &user); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	if user.ID != agentUserID {
		t.Fatalf("expected agent user, got %q", user.ID)
	}
}

func TestMCP_Resource_Channel(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res, err := cs.ReadResource(ctx, &mcpsdk.ReadResourceParams{
		URI: "chit://channels/" + channelID,
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}

	var ch model.Channel
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &ch); err != nil {
		t.Fatalf("unmarshal channel: %v", err)
	}
	if ch.ID != channelID {
		t.Fatalf("unexpected channel: %+v", ch)
	}
}

func TestMCP_Resource_RecentPosts(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res, err := cs.ReadResource(ctx, &mcpsdk.ReadResourceParams{
		URI: "chit://channels/" + channelID + "/recent",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}

	var postList model.PostList
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &postList); err != nil {
		t.Fatalf("unmarshal posts: %v", err)
	}
	if len(postList.Order) < 1 {
		t.Fatal("expected at least 1 recent post")
	}
}

func TestMCP_Resource_Thread(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res, err := cs.ReadResource(ctx, &mcpsdk.ReadResourceParams{
		URI: "chit://threads/" + rootPostID,
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}

	var postList model.PostList
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &postList); err != nil {
		t.Fatalf("unmarshal thread: %v", err)
	}
	if len(postList.Order) < 2 {
		t.Fatalf("expected at least 2 posts in thread, got %d", len(postList.Order))
	}
}

func TestMCP_Resource_TeamChannels(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res, err := cs.ReadResource(ctx, &mcpsdk.ReadResourceParams{
		URI: "chit://teams/" + teamID + "/channels",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}

	var channels []*model.Channel
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &channels); err != nil {
		t.Fatalf("unmarshal channels: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
}

// ─── Prompt tests ────────────────────────────────────────────────

func TestMCP_Prompt_SummarizeChannel(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res, err := cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{
		Name: "summarize_channel",
		Arguments: map[string]string{
			"channel_id": channelID,
		},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(res.Messages) == 0 {
		t.Fatal("expected at least one prompt message")
	}
	tc, ok := res.Messages[0].Content.(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Messages[0].Content)
	}
	if !strings.Contains(tc.Text, "General") {
		t.Fatal("expected prompt to contain channel display name")
	}
}

func TestMCP_Prompt_DraftReply(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	res, err := cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{
		Name: "draft_reply",
		Arguments: map[string]string{
			"post_id": rootPostID,
		},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(res.Messages) == 0 {
		t.Fatal("expected at least one prompt message")
	}
	tc, ok := res.Messages[0].Content.(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Messages[0].Content)
	}
	if !strings.Contains(tc.Text, "Draft a reply") {
		t.Fatal("expected prompt to contain 'Draft a reply'")
	}
}
