package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/infrashift/chit/internal/api"
	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/jobs"
	"github.com/infrashift/chit/internal/jobs/workers"
	"github.com/infrashift/chit/internal/pubsub"
	"github.com/infrashift/chit/internal/store/sqlstore"
	"github.com/infrashift/chit/internal/websocket"
)

// Server holds all components and manages their lifecycle.
type Server struct {
	config    *config.Config
	store     *sqlstore.SqlStore
	hub       *websocket.Hub
	pubsub    pubsub.PubSub
	app       *app.App
	scheduler *jobs.Scheduler
	httpSrv   *http.Server
	webhookCh chan *command.WebhookEvent
}

// New creates a Server from configuration.
func New(cfg *config.Config) (*Server, error) {
	s := &Server{config: cfg}

	if err := s.initLogging(); err != nil {
		return nil, err
	}

	if err := s.initStore(); err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	s.initHub()

	if err := s.initPubSub(); err != nil {
		return nil, fmt.Errorf("init pubsub: %w", err)
	}

	s.initApp()

	// After initApp, because it needs the App and its store; before initHTTP,
	// because a request that arrives first would be refused as an unknown
	// client and the caller would see a 401 that later becomes a 200 for no
	// reason it can observe.
	if err := s.ensureMachineActors(); err != nil {
		return nil, fmt.Errorf("ensure machine actors: %w", err)
	}

	s.initCommands()
	s.initJobs()
	s.initHTTP()

	return s, nil
}

func (s *Server) initLogging() error {
	var handler slog.Handler
	level := slog.LevelInfo
	switch s.config.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	if s.config.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
	return nil
}

func (s *Server) initStore() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	store, err := sqlstore.New(ctx, s.config.DatabaseURL, s.config.DBMaxOpenConn, s.config.DBMaxIdleConn)
	if err != nil {
		return err
	}
	s.store = store
	return nil
}

func (s *Server) initHub() {
	s.hub = websocket.NewHub(hubMembershipAdapter{store: s.store})
}

// hubMembershipAdapter exposes channel and team memberships to the websocket
// hub without giving it the whole store.
type hubMembershipAdapter struct {
	store *sqlstore.SqlStore
}

func (a hubMembershipAdapter) GetChannelIDsForUser(userID string) ([]string, error) {
	// Hub membership loads are server-internal work, not tied to a request.
	return a.store.Channel().GetChannelIDsForUser(context.Background(), userID)
}

func (a hubMembershipAdapter) GetTeamIDsForUser(userID string) ([]string, error) {
	teams, err := a.store.Team().GetTeamsForUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(teams))
	for i, t := range teams {
		ids[i] = t.ID
	}
	return ids, nil
}

func (s *Server) initPubSub() error {
	switch s.config.PubSubBackend {
	case "nats":
		ps, err := pubsub.NewNatsPubSub(s.config.NatsURL)
		if err != nil {
			return err
		}
		s.pubsub = ps
	default:
		ps, err := pubsub.NewPGNotify(s.store.Pool())
		if err != nil {
			return err
		}
		s.pubsub = ps
	}
	return nil
}

func (s *Server) initApp() {
	s.app = app.New(s.store, s.hub, s.pubsub, s.config)
}

func (s *Server) initCommands() {
	cueCfg, err := command.LoadCUE(s.config.CommandsCUEDir)
	if err != nil {
		slog.Warn("slash commands disabled: failed to load CUE definitions",
			"dir", s.config.CommandsCUEDir, "error", err)
		return
	}

	reg := command.NewRegistry(cueCfg.Commands)
	s.app.CommandRegistry = reg

	// Register built-in handlers.
	handlers := make(map[string]command.Handler)
	handlers["help"] = command.HandlerFunc(func(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
		var text string
		for _, c := range reg.All() {
			text += fmt.Sprintf("- `/%s` — %s\n", c.Slug, c.Description)
		}
		if text == "" {
			text = "No commands available."
		}
		return &command.CommandResult{ResponseText: text}, nil
	})
	handlers["invite"] = command.HandlerFunc(func(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
		return s.app.HandleInvite(ctx, actorID, channelID, args)
	})
	handlers["kick"] = command.HandlerFunc(func(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
		return s.app.HandleKick(ctx, actorID, channelID, args)
	})

	handlers["topic"] = command.HandlerFunc(func(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
		return s.app.HandleTopic(ctx, actorID, channelID, args)
	})

	s.app.CommandHandlers = handlers

	// Audit logger.
	al, err := command.NewAuditLogger(s.config.AuditLogPath)
	if err != nil {
		slog.Warn("failed to create audit logger, using stderr", "error", err)
		al, _ = command.NewAuditLogger("-")
	}
	s.app.AuditLogger = al

	// Webhook channel.
	if s.config.WebhookEnabled && s.config.WebhookURL != "" {
		ch := make(chan *command.WebhookEvent, s.config.WebhookQueueSize)
		s.webhookCh = ch
		s.app.WebhookCh = ch
	}

	slog.Info("slash commands initialized",
		"commands", len(cueCfg.Commands),
		"roles", len(cueCfg.Roles),
		"webhook_enabled", s.config.WebhookEnabled)
}

func (s *Server) initJobs() {
	s.scheduler = jobs.NewScheduler()
	indexer := workers.NewSearchIndexer(
		s.config.ZincSearchURL,
		s.config.ZincSearchUser,
		s.config.ZincSearchPassword,
	).WithPostStore(s.store.Post())
	s.scheduler.AddWorker(indexer)

	if s.webhookCh != nil {
		dispatcher := workers.NewWebhookDispatcher(
			s.webhookCh,
			s.config.WebhookURL,
			s.config.WebhookSecret,
			s.config.WebhookWorkerCount,
			time.Duration(s.config.WebhookTimeoutSec)*time.Second,
		)
		s.scheduler.AddWorker(dispatcher)
	}
}

func (s *Server) initHTTP() {
	handler := api.New(s.app)
	s.httpSrv = &http.Server{
		Addr:         s.config.ListenAddress,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// Start begins serving and blocks until a shutdown signal is received.
// ensureMachineActors reconciles the declared non-human callers at boot.
//
// FATAL ON FAILURE, deliberately. These rows are the difference between a
// machine actor being authorized and being a 401, and a server that starts
// without them looks entirely healthy while refusing every client_credentials
// token — which is a much longer debugging session than a refusal to start.
func (s *Server) ensureMachineActors() error {
	actors, err := app.ParseMachineActors(s.config.MachineActors)
	if err != nil {
		return err
	}
	if len(actors) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.app.EnsureMachineActors(ctx, actors)
}

func (s *Server) Start() error {
	s.scheduler.Start()

	slog.Info("server starting", "address", s.config.ListenAddress)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig)
	}

	return s.Shutdown()
}

// Shutdown gracefully shuts down all components.
func (s *Server) Shutdown() error {
	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpSrv.Shutdown(ctx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}

	s.hub.Stop()
	s.scheduler.Stop()

	if s.pubsub != nil {
		if err := s.pubsub.Close(); err != nil {
			slog.Error("pubsub close error", "error", err)
		}
	}

	if s.app.AuditLogger != nil {
		if err := s.app.AuditLogger.Close(); err != nil {
			slog.Error("audit logger close error", "error", err)
		}
	}

	s.store.Close()

	slog.Info("server shutdown complete")
	return nil
}
