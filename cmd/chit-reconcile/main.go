package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/keto"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// LoadWithoutDatabase, because this tool has no database. It reads CUE from
	// disk and writes tuples to Keto over HTTP; cfg.DatabaseURL is never read.
	// Load() would refuse to start without CHIT_DATABASE_URL, which is what made
	// the chit-server seed job fail after successfully registering every OAuth2
	// client — a batch job asked for a credential it would not have used.
	cfg, err := config.LoadWithoutDatabase()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	assert.That(cfg != nil, "chit-reconcile: nil config without an error")
	cueDir := cfg.CommandsCUEDir
	if cueDir == "" {
		cueDir = "auth"
	}
	assert.That(cueDir != "", "chit-reconcile: no CUE directory")

	slog.Info("loading CUE definitions", "dir", cueDir)
	cueCfg, err := command.LoadCUE(cueDir)
	if err != nil {
		log.Fatalf("failed to load CUE: %v", err)
	}

	assert.That(cueCfg != nil, "chit-reconcile: nil CUE config without an error")
	slog.Info("CUE loaded",
		"commands", len(cueCfg.Commands),
		"roles", len(cueCfg.Roles),
		"actors", len(cueCfg.Actors))

	// The read URL lists what is already granted, so grants the CUE no longer
	// declares can be revoked.
	rec := command.NewReconciler(keto.New(cfg.KetoReadURL, cfg.KetoWriteURL, &http.Client{Timeout: 10 * time.Second}))

	assert.That(rec != nil, "chit-reconcile: nil reconciler")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	err = rec.Reconcile(ctx, cueCfg)
	cancel()
	if err != nil {
		log.Fatalf("reconcile failed: %v", err)
	}

	slog.Info("reconciliation complete")
}
