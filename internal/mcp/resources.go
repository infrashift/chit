package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/assert"
)

func (s *ChitMCPServer) registerResources() {
	assert.That(s.server != nil, "ChitMCPServer.registerResources: nil MCP server")
	assert.That(s.client != nil, "ChitMCPServer.registerResources: resources registered without a chitd client")
	// Static resource: agent's own identity
	s.server.AddResource(&mcp.Resource{
		URI:         "chit://agent/identity",
		Name:        "Agent Identity",
		Description: "The agent's own user profile in the Chit system",
		MIMEType:    "application/json",
	}, s.handleAgentIdentity)

	// Template: channel metadata
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "chit://channels/{channel_id}",
		Name:        "Channel",
		Description: "Metadata for a specific channel",
		MIMEType:    "application/json",
	}, s.handleChannelResource)

	// Template: recent posts in a channel
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "chit://channels/{channel_id}/recent",
		Name:        "Recent Channel Posts",
		Description: "The 20 most recent posts in a channel",
		MIMEType:    "application/json",
	}, s.handleChannelRecentResource)

	// Template: full thread context
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "chit://threads/{post_id}",
		Name:        "Thread",
		Description: "Full thread context including root post and all replies",
		MIMEType:    "application/json",
	}, s.handleThreadResource)

	// Template: team's channel listing
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "chit://teams/{team_id}/channels",
		Name:        "Team Channels",
		Description: "All channels in a team",
		MIMEType:    "application/json",
	}, s.handleTeamChannelsResource)
}

func (s *ChitMCPServer) handleAgentIdentity(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleAgentIdentity: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleAgentIdentity: nil chitd client")
	assert.That(req != nil && req.Params != nil, "ChitMCPServer.handleAgentIdentity: nil request")
	assert.That(strings.HasPrefix(req.Params.URI, "chit://agent/identity"), "ChitMCPServer.handleAgentIdentity: URI outside the registered template")
	user, err := s.client.Me(ctx)
	if err != nil {
		return nil, fmt.Errorf("get agent user: %w", err)
	}
	assert.That(user != nil, "ChitMCPServer.handleAgentIdentity: nil user without an error")
	data, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("marshal agent user: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *ChitMCPServer) handleChannelResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleChannelResource: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleChannelResource: nil chitd client")
	assert.That(req != nil && req.Params != nil, "ChitMCPServer.handleChannelResource: nil request")
	assert.That(strings.HasPrefix(req.Params.URI, "chit://channels/"), "ChitMCPServer.handleChannelResource: URI outside the registered template")
	channelID := extractURIParam(req.Params.URI, "chit://channels/", "")
	if channelID == "" {
		return nil, fmt.Errorf("invalid channel URI: %s", req.Params.URI)
	}
	channel, err := s.client.GetChannel(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	assert.That(channel != nil, "ChitMCPServer.handleChannelResource: nil channel without an error")
	data, err := json.Marshal(channel)
	if err != nil {
		return nil, fmt.Errorf("marshal channel: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *ChitMCPServer) handleChannelRecentResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleChannelRecentResource: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleChannelRecentResource: nil chitd client")
	assert.That(req != nil && req.Params != nil, "ChitMCPServer.handleChannelRecentResource: nil request")
	assert.That(strings.HasPrefix(req.Params.URI, "chit://channels/"), "ChitMCPServer.handleChannelRecentResource: URI outside the registered template")
	// The URI has the form chit://channels/{channel_id}/recent.
	channelID := extractURIParam(req.Params.URI, "chit://channels/", "/recent")
	if channelID == "" {
		return nil, fmt.Errorf("invalid channel recent URI: %s", req.Params.URI)
	}
	posts, err := s.client.GetChannelPosts(ctx, channelID, 0, 20, 0)
	if err != nil {
		return nil, fmt.Errorf("get channel posts: %w", err)
	}
	assert.That(posts != nil, "ChitMCPServer.handleChannelRecentResource: nil post list without an error")
	data, err := json.Marshal(viewPostList(posts))
	if err != nil {
		return nil, fmt.Errorf("marshal channel posts: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *ChitMCPServer) handleThreadResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleThreadResource: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleThreadResource: nil chitd client")
	assert.That(req != nil && req.Params != nil, "ChitMCPServer.handleThreadResource: nil request")
	assert.That(strings.HasPrefix(req.Params.URI, "chit://threads/"), "ChitMCPServer.handleThreadResource: URI outside the registered template")
	postID := extractURIParam(req.Params.URI, "chit://threads/", "")
	if postID == "" {
		return nil, fmt.Errorf("invalid thread URI: %s", req.Params.URI)
	}
	posts, err := s.client.GetThread(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("get thread: %w", err)
	}
	data, err := json.Marshal(viewPosts(posts))
	if err != nil {
		return nil, fmt.Errorf("marshal thread: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *ChitMCPServer) handleTeamChannelsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	assert.That(ctx != nil, "ChitMCPServer.handleTeamChannelsResource: nil context")
	assert.That(s.client != nil, "ChitMCPServer.handleTeamChannelsResource: nil chitd client")
	assert.That(req != nil && req.Params != nil, "ChitMCPServer.handleTeamChannelsResource: nil request")
	assert.That(strings.HasPrefix(req.Params.URI, "chit://teams/"), "ChitMCPServer.handleTeamChannelsResource: URI outside the registered template")
	// The URI has the form chit://teams/{team_id}/channels.
	teamID := extractURIParam(req.Params.URI, "chit://teams/", "/channels")
	if teamID == "" {
		return nil, fmt.Errorf("invalid team channels URI: %s", req.Params.URI)
	}
	channels, err := s.client.GetChannelsForTeam(ctx, teamID, 0, 200)
	if err != nil {
		return nil, fmt.Errorf("get team channels: %w", err)
	}
	data, err := json.Marshal(channels)
	if err != nil {
		return nil, fmt.Errorf("marshal team channels: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

// extractURIParam extracts the dynamic segment from a URI given a prefix and suffix.
// For example: extractURIParam("chit://channels/abc-123/recent", "chit://channels/", "/recent") => "abc-123"
func extractURIParam(uri, prefix, suffix string) string {
	assert.That(strings.HasPrefix(prefix, "chit://") && strings.HasSuffix(prefix, "/"), "extractURIParam: prefix is not a chit:// path")
	assert.That(suffix == "" || strings.HasPrefix(suffix, "/"), "extractURIParam: suffix is not a path segment")
	s := strings.TrimPrefix(uri, prefix)
	if suffix != "" {
		s = strings.TrimSuffix(s, suffix)
	}
	// Remove any trailing slashes
	s = strings.TrimRight(s, "/")
	if s == "" || s == uri {
		return ""
	}
	assert.That(!strings.HasSuffix(s, "/") && len(s) < len(uri), "extractURIParam: parameter keeps a slash or the prefix")
	return s
}
