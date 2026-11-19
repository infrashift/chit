package mcp

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/assert"
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
	assert.That(client != nil, "New: nil chitd client")
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

	assert.That(s.server != nil, "New: nil MCP server")
	assert.That(s.eventBuffer != nil, "New: nil event buffer")
	return s
}

// Run starts the MCP server on stdio transport, blocking until the client
// disconnects or the context is cancelled.
func (s *ChitMCPServer) Run(ctx context.Context) error {
	assert.That(ctx != nil, "ChitMCPServer.Run: nil context")
	assert.That(s.server != nil, "ChitMCPServer.Run: nil MCP server")
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
	assert.That(ctx != nil, "ChitMCPServer.StartEventFeed: nil context")
	assert.That(s.client != nil, "ChitMCPServer.StartEventFeed: nil chitd client")
	assert.That(s.eventBuffer != nil, "ChitMCPServer.StartEventFeed: nil event buffer")
	go s.client.Listen(ctx, eventFeed{buffer: s.eventBuffer})
}

// eventFeed buffers the events of chitd's WebSocket for get_new_events.
type eventFeed struct {
	buffer *EventBuffer
}

// HandleEvent buffers event, dropping ephemeral command_response events.
func (f eventFeed) HandleEvent(event *model.WebSocketEvent) {
	assert.That(event != nil, "eventFeed.HandleEvent: nil event")
	assert.That(f.buffer != nil, "eventFeed.HandleEvent: nil buffer")
	if event.Event == model.WebSocketEventCommandResponse {
		return
	}
	f.buffer.Push(event)
}
