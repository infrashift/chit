// chit-claude bridges Chit channels to headless Claude Code sessions.
// See internal/bridge for the architecture.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/infrashift/chit/internal/bridge"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	if err := run(); err != nil {
		slog.Error("bridge error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := bridge.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("bridge: shutdown signal received")
		cancel()
	}()

	return bridge.New(cfg).Run(ctx)
}
