package main

import (
	"log"
	"log/slog"

	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	slog.Info("chit server starting")
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
