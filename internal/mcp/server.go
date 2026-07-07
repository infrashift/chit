package mcp

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/chitclient"
	"github.com/infrashift/chit/internal/model"
)

// ChitMCPServer bridges MCP clients to chitd over its HTTP API and
// WebSocket, acting as the configured agent user.
type ChitMCPServer struct {
	server      *mcp.Server
	client      *chitclient.Client
	eventBuffer *EventBuffer
}

// New creates a ChitMCPServer, registering all tools, resources, and prompts.
func New(client *chitclient.Client) *ChitMCPServer {
	s := &ChitMCPServer{
		server: mcp.NewServer(&mcp.Implementation{
			Name:    "chit-mcp",
			Version: "0.1.0",
		}, &mcp.ServerOptions{
			InitializedHandler: func(ctx context.Context, req *mcp.InitializedRequest) {
				slog.Info("mcp: client initialized")
			},
		}),
		client:      client,
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
	slog.Info("mcp: starting stdio server")
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// StartEventFeed consumes chitd's WebSocket in the background and buffers
// events for agents to poll via get_new_events. chitd's hub already filters
// events server-side (user-targeted events reach only their target, channel
// events only channel members), so no client-side filtering is needed.
// Ephemeral command_response events are dropped: they answer someone else's
// slash command and were never part of the agent feed.
func (s *ChitMCPServer) StartEventFeed(ctx context.Context) {
	go s.client.Listen(ctx, func(event *model.WebSocketEvent) {
		if event.Event == model.WebSocketEventCommandResponse {
			return
		}
		s.eventBuffer.Push(event)
	})
}
