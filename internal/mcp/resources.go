package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/model"
)

func (s *ChitMCPServer) registerResources() {
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
	user, err := s.client.Me(ctx)
	if err != nil {
		return nil, fmt.Errorf("get agent user: %w", err)
	}
	data, _ := json.Marshal(user)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *ChitMCPServer) handleChannelResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	channelID := extractURIParam(req.Params.URI, "chit://channels/", "")
	if channelID == "" {
		return nil, fmt.Errorf("invalid channel URI: %s", req.Params.URI)
	}
	channel, err := s.client.GetChannel(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	data, _ := json.Marshal(channel)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *ChitMCPServer) handleChannelRecentResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// The URI has the form chit://channels/{channel_id}/recent.
	channelID := extractURIParam(req.Params.URI, "chit://channels/", "/recent")
	if channelID == "" {
		return nil, fmt.Errorf("invalid channel recent URI: %s", req.Params.URI)
	}
	posts, err := s.client.GetChannelPosts(ctx, channelID, 0, 20, 0)
	if err != nil {
		return nil, fmt.Errorf("get channel posts: %w", err)
	}
	data, _ := json.Marshal(posts)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *ChitMCPServer) handleThreadResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	postID := extractURIParam(req.Params.URI, "chit://threads/", "")
	if postID == "" {
		return nil, fmt.Errorf("invalid thread URI: %s", req.Params.URI)
	}
	posts, err := s.client.GetThread(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("get thread: %w", err)
	}
	data, _ := json.Marshal(&model.PostList{Order: posts})
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *ChitMCPServer) handleTeamChannelsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// The URI has the form chit://teams/{team_id}/channels.
	teamID := extractURIParam(req.Params.URI, "chit://teams/", "/channels")
	if teamID == "" {
		return nil, fmt.Errorf("invalid team channels URI: %s", req.Params.URI)
	}
	channels, err := s.client.GetChannelsForTeam(ctx, teamID, 0, 200)
	if err != nil {
		return nil, fmt.Errorf("get team channels: %w", err)
	}
	data, _ := json.Marshal(channels)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

// extractURIParam extracts the dynamic segment from a URI given a prefix and suffix.
// For example: extractURIParam("chit://channels/abc-123/recent", "chit://channels/", "/recent") => "abc-123"
func extractURIParam(uri, prefix, suffix string) string {
	s := strings.TrimPrefix(uri, prefix)
	if suffix != "" {
		s = strings.TrimSuffix(s, suffix)
	}
	// Remove any trailing slashes
	s = strings.TrimRight(s, "/")
	if s == "" || s == uri {
		return ""
	}
	return s
}
