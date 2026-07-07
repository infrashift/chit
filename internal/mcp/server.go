package mcp

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
)

// ChitMCPServer wraps the MCP server with references to the Chit App layer
// and the agent's identity.
type ChitMCPServer struct {
	server      *mcp.Server
	app         *app.App
	agentUserID string
	eventBuffer *EventBuffer
}

// New creates a ChitMCPServer, registering all tools, resources, and prompts.
func New(a *app.App, agentUserID string) *ChitMCPServer {
	s := &ChitMCPServer{
		server: mcp.NewServer(&mcp.Implementation{
			Name:    "chit-mcp",
			Version: "0.1.0",
		}, &mcp.ServerOptions{
			InitializedHandler: func(ctx context.Context, req *mcp.InitializedRequest) {
				slog.Info("mcp: client initialized")
			},
		}),
		app:         a,
		agentUserID: agentUserID,
		eventBuffer: NewEventBuffer(),
	}

	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s
}

// Run starts the MCP server on stdio transport, blocking until the client
// disconnects or the context is cancelled.
func (s *ChitMCPServer) Run(ctx context.Context) error {
	slog.Info("mcp: starting stdio server", "agent_user_id", s.agentUserID)
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// StartEventFeed subscribes the event buffer to the cross-process event topic
// published by chitd, so get_new_events returns real-time activity. Events are
// filtered to what the agent may see: user-targeted events must target the
// agent, and channel-scoped events require channel membership.
func (s *ChitMCPServer) StartEventFeed(ctx context.Context, ps pubsub.PubSub) error {
	return ps.Subscribe(ctx, pubsub.TopicEvents, func(data []byte) {
		var env pubsub.EventEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			slog.Warn("mcp: invalid event envelope", "error", err)
			return
		}

		if env.TargetUserID != "" && env.TargetUserID != s.agentUserID {
			return
		}
		if env.ChannelID != "" && env.TargetUserID == "" {
			if _, err := s.app.Store.Channel().GetMember(ctx, env.ChannelID, s.agentUserID); err != nil {
				return
			}
		}

		event := &model.WebSocketEvent{
			Event: env.Event,
			Data:  map[string]any{},
		}
		if env.PostID != "" {
			event.Data["post_id"] = env.PostID
		}
		if env.ChannelID != "" {
			event.Data["channel_id"] = env.ChannelID
		}
		if env.TeamID != "" {
			event.Data["team_id"] = env.TeamID
		}
		s.eventBuffer.Push(event)
	})
}
