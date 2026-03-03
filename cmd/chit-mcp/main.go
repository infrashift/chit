package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/mcp"
	"github.com/infrashift/chit/internal/pubsub"
	"github.com/infrashift/chit/internal/store/sqlstore"
	"github.com/infrashift/chit/internal/websocket"
)

func main() {
	// MCP servers use stdio for transport; redirect slog to stderr so
	// log output does not corrupt the JSON-RPC stream.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	agentUserID := cfg.MCPAgentUserID
	if agentUserID == "" {
		log.Fatal("CHIT_MCP_AGENT_USER_ID is required")
	}

	// Initialize the same Store + App stack used by the HTTP server.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := sqlstore.New(ctx, cfg.DatabaseURL, cfg.DBMaxOpenConn, cfg.DBMaxIdleConn)
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	hub := websocket.NewHub()

	var ps pubsub.PubSub
	switch cfg.PubSubBackend {
	case "nats":
		ps, err = pubsub.NewNatsPubSub(cfg.NatsURL)
		if err != nil {
			log.Fatalf("failed to init nats pubsub: %v", err)
		}
	default:
		ps, err = pubsub.NewPGNotify(store.Pool())
		if err != nil {
			log.Fatalf("failed to init pg pubsub: %v", err)
		}
	}
	defer ps.Close()

	a := app.New(store, hub, ps, cfg)

	server := mcp.New(a, agentUserID)

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("mcp: shutdown signal received")
		cancel()
	}()

	slog.Info("chit-mcp starting", "agent_user_id", agentUserID)
	if err := server.Run(ctx); err != nil {
		log.Fatalf("mcp server error: %v", err)
	}
}
