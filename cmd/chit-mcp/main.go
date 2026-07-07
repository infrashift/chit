package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/infrashift/chit/internal/chitclient"
	"github.com/infrashift/chit/internal/mcp"
	"github.com/infrashift/chit/internal/model"
)

func main() {
	// MCP servers use stdio for transport; redirect slog to stderr so
	// log output does not corrupt the JSON-RPC stream.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	if err := run(); err != nil {
		log.Fatalf("chit-mcp: %v", err)
	}
}

func run() error {
	cfg, err := mcp.LoadConfig()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := chitclient.New(cfg.ServerURL, cfg.AgentKratosID, cfg.ProxySecret)

	// Resolve the agent's identity through chitd (provisioning it on first
	// contact); warn when it is not marked as an agent actor so audit logs
	// stay truthful.
	if me, merr := client.Me(ctx); merr != nil {
		slog.Warn("mcp: could not resolve agent identity; tool calls will fail until chitd is reachable",
			"server_url", cfg.ServerURL, "error", merr)
	} else if me.ActorType != model.ActorTypeAgent {
		slog.Warn("mcp: agent user is not marked actor_type=agent; audit logs will record it as a regular user",
			"user_id", me.ID, "actor_type", me.ActorType)
	} else {
		slog.Info("mcp: acting as agent", "user_id", me.ID, "username", me.Username)
	}

	server := mcp.New(client)

	// Feed real-time events from chitd's WebSocket into the agent's poll buffer.
	server.StartEventFeed(ctx)

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("mcp: shutdown signal received")
		cancel()
	}()

	slog.Info("chit-mcp starting", "server_url", cfg.ServerURL)
	return server.Run(ctx)
}
