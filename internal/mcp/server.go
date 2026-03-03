package mcp

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/app"
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
