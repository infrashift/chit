package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/assert"
)

func (s *ChitMCPServer) registerPrompts() {
	assert.That(s.server != nil, "ChitMCPServer.registerPrompts: nil MCP server")
	assert.That(s.client != nil, "ChitMCPServer.registerPrompts: prompts registered without a chitd client")
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "summarize_channel",
		Description: "Generate a summary of recent activity in a channel",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "channel_id",
				Description: "The channel to summarize",
				Required:    true,
			},
			{
				Name:        "num_posts",
				Description: "Number of recent posts to include (default: 50, at most 200)",
				Required:    false,
			},
		},
	}, s.handleSummarizeChannel)

	s.server.AddPrompt(&mcp.Prompt{
		Name:        "draft_reply",
		Description: "Draft a reply to a thread based on the full conversation context",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "post_id",
				Description: "Root post ID of the thread to reply to",
				Required:    true,
			},
			{
				Name:        "instructions",
				Description: "Additional instructions for the reply (tone, focus, etc.)",
				Required:    false,
			},
		},
	}, s.handleDraftReply)
}

func (s *ChitMCPServer) handleSummarizeChannel(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleSummarizeChannel: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleSummarizeChannel: nil chitd client")
	assert.That(req != nil && req.Params != nil, "ChitMCPServer.handleSummarizeChannel: nil request")
	channelID := req.Params.Arguments["channel_id"]
	if channelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}

	perPage := 50
	if n := req.Params.Arguments["num_posts"]; n != "" {
		parsed, err := strconv.Atoi(n)
		if err != nil || parsed < 1 || parsed > maxPerPage {
			return nil, fmt.Errorf("num_posts must be an integer from 1 to %d, got %q", maxPerPage, n)
		}
		perPage = parsed
	}
	assert.That(perPage >= 1 && perPage <= maxPerPage, "ChitMCPServer.handleSummarizeChannel: num_posts not bounded")

	channel, err := s.client.GetChannel(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	assert.That(channel != nil, "ChitMCPServer.handleSummarizeChannel: nil channel without an error")

	posts, err := s.client.GetChannelPosts(ctx, channelID, 0, perPage, 0)
	if err != nil {
		return nil, fmt.Errorf("get channel posts: %w", err)
	}

	postsJSON, err := json.Marshal(viewPostList(posts))
	if err != nil {
		return nil, fmt.Errorf("marshal channel posts: %w", err)
	}

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Summarize recent activity in #%s", channel.DisplayName),
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(
						"Summarize the recent activity in channel #%s (%s).\n\nChannel purpose: %s\n\nRecent posts (JSON):\n%s\n\nProvide a concise summary of the key topics discussed, any decisions made, and action items mentioned.",
						channel.DisplayName,
						channel.Name,
						channel.Purpose,
						string(postsJSON),
					),
				},
			},
		},
	}, nil
}

func (s *ChitMCPServer) handleDraftReply(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleDraftReply: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleDraftReply: nil chitd client")
	assert.That(req != nil && req.Params != nil, "ChitMCPServer.handleDraftReply: nil request")
	postID := req.Params.Arguments["post_id"]
	if postID == "" {
		return nil, fmt.Errorf("post_id is required")
	}

	instructions := req.Params.Arguments["instructions"]

	posts, err := s.client.GetThread(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("get thread: %w", err)
	}

	threadJSON, err := json.Marshal(viewPosts(posts))
	if err != nil {
		return nil, fmt.Errorf("marshal thread: %w", err)
	}
	assert.That(len(threadJSON) > 0, "ChitMCPServer.handleDraftReply: empty thread encoding without an error")

	promptText := fmt.Sprintf(
		"Draft a reply to the following thread conversation.\n\nThread (JSON):\n%s",
		string(threadJSON),
	)
	if instructions != "" {
		promptText += fmt.Sprintf("\n\nAdditional instructions: %s", instructions)
	}
	promptText += "\n\nDraft a helpful, contextually appropriate reply. Return only the reply text, without any wrapper or explanation."

	return &mcp.GetPromptResult{
		Description: "Draft a reply to a thread",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: promptText},
			},
		},
	}, nil
}
