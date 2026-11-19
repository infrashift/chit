package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/infrashift/chit/internal/api"
	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/blob"
	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/errlog"
	"github.com/infrashift/chit/internal/jobs"
	"github.com/infrashift/chit/internal/jobs/workers"
	"github.com/infrashift/chit/internal/memberhook"
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
	assert.That(cfg != nil, "server.New: nil config")
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

	if err := s.initBlobs(); err != nil {
		return nil, fmt.Errorf("init attachments: %w", err)
	}

	// After initApp, because it needs the App and its store; before initHTTP,
	// because a request that arrives first would be refused as an unknown
	// client and the caller would see a 401 that later becomes a 200 for no
	// reason it can observe.
	if err := s.ensureMachineActors(); err != nil {
		return nil, fmt.Errorf("ensure machine actors: %w", err)
	}

	s.initCommands()
	if err := s.initMembershipHook(); err != nil {
		return nil, fmt.Errorf("init membership hook: %w", err)
	}
	s.initJobs()
	s.initHTTP()
	assert.That(s.app != nil && s.scheduler != nil && s.httpSrv != nil, "server.New: a component was not built")

	return s, nil
}

func (s *Server) initLogging() error {
	assert.That(s.config != nil, "Server.initLogging: nil config")
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
	assert.That(handler != nil, "Server.initLogging: no handler chosen")
	slog.SetDefault(slog.New(handler))
	return nil
}

func (s *Server) initStore() error {
	assert.That(s.config != nil, "Server.initStore: nil config")
	assert.That(s.store == nil, "Server.initStore: store already open")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	store, err := sqlstore.New(ctx, s.config.DatabaseURL, s.config.DBMaxOpenConn, s.config.DBMaxIdleConn)
	if err != nil {
		return err
	}
	assert.That(store != nil, "Server.initStore: nil store without an error")
	s.store = store
	return nil
}

func (s *Server) initHub() {
	assert.That(s.store != nil, "Server.initHub: store not open")
	s.hub = websocket.NewHub(hubMembershipAdapter{store: s.store})
	assert.That(s.hub != nil, "Server.initHub: nil hub")
}

// hubMembershipAdapter exposes channel and team memberships to the websocket
// hub without giving it the whole store.
type hubMembershipAdapter struct {
	store *sqlstore.SqlStore
}

// membershipLoadTimeout bounds a hub membership load. The loads are not tied
// to a request, and without a deadline a hung query left the user marked as
// loading forever, so their load was never retried.
const membershipLoadTimeout = 10 * time.Second

func (a hubMembershipAdapter) GetChannelIDsForUser(userID string) ([]string, error) {
	assert.That(a.store != nil, "hubMembershipAdapter.GetChannelIDsForUser: nil store")
	assert.That(userID != "", "hubMembershipAdapter.GetChannelIDsForUser: empty user id")
	ctx, cancel := context.WithTimeout(context.Background(), membershipLoadTimeout)
	defer cancel()
	return a.store.Channel().GetChannelIDsForUser(ctx, userID)
}

func (a hubMembershipAdapter) GetTeamIDsForUser(userID string) ([]string, error) {
	assert.That(a.store != nil, "hubMembershipAdapter.GetTeamIDsForUser: nil store")
	assert.That(userID != "", "hubMembershipAdapter.GetTeamIDsForUser: empty user id")
	ctx, cancel := context.WithTimeout(context.Background(), membershipLoadTimeout)
	defer cancel()
	teams, err := a.store.Team().GetTeamsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(teams))
	for i, t := range teams {
		ids[i] = t.ID
	}
	assert.That(len(ids) == len(teams), "hubMembershipAdapter.GetTeamIDsForUser: lost a team id")
	return ids, nil
}

func (s *Server) initPubSub() error {
	assert.That(s.store != nil, "Server.initPubSub: store not open")
	switch s.config.PubSubBackend {
	case "nats":
		ps, err := pubsub.NewNatsPubSub(s.config.NatsURL)
		if err != nil {
			return err
		}
		s.pubsub = ps
	default: // "pgnotify"; config validation refuses anything else
		ps, err := pubsub.NewPGNotify(s.store.Pool())
		if err != nil {
			return err
		}
		s.pubsub = ps
	}
	assert.That(s.pubsub != nil, "Server.initPubSub: nil pubsub without an error")
	return nil
}

func (s *Server) initApp() {
	assert.That(s.store != nil && s.hub != nil && s.pubsub != nil, "Server.initApp: a dependency is not ready")
	s.app = app.New(s.store, s.hub, s.pubsub, s.config)
	assert.That(s.app != nil, "Server.initApp: nil app")
}

// initBlobs connects the attachment object store when CHIT_S3_ENDPOINT is
// set, and creates the bucket if it is missing. The store may still be
// starting beside chitd (the deploy manifests run SeaweedFS in the same
// pod), so it is retried for blobStartTimeout before chitd gives up.
func (s *Server) initBlobs() error {
	if s.config.S3Endpoint == "" {
		slog.Info("attachments are off: CHIT_S3_ENDPOINT is not set")
		return nil
	}
	store, err := blob.NewS3(&blob.S3Config{
		Endpoint:        s.config.S3Endpoint,
		Bucket:          s.config.S3Bucket,
		Region:          s.config.S3Region,
		AccessKeyID:     s.config.S3AccessKeyID,
		SecretAccessKey: s.config.S3SecretAccessKey,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), blobStartTimeout)
	defer cancel()
	for {
		err = store.EnsureBucket(ctx)
		if err == nil {
			break
		}
		slog.Warn("attachments: object store not ready; retrying", "endpoint", s.config.S3Endpoint, "error", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("bucket %s at %s: %w", s.config.S3Bucket, s.config.S3Endpoint, err)
		case <-time.After(2 * time.Second):
		}
	}
	s.app.Blobs = store
	slog.Info("attachments are on", "endpoint", s.config.S3Endpoint, "bucket", s.config.S3Bucket)
	return nil
}

// initMembershipHook connects the membership policy hook when
// CHIT_MEMBERSHIP_HOOK_URL is set. Its answers are audited, so it makes sure
// there is an audit log even when slash commands, which open it, are off.
func (s *Server) initMembershipHook() error {
	if s.config.MembershipHookURL == "" {
		slog.Info("membership hook is off: CHIT_MEMBERSHIP_HOOK_URL is not set")
		return nil
	}
	hook, err := memberhook.New(&memberhook.Config{
		URL:          s.config.MembershipHookURL,
		Timeout:      s.config.MembershipHookTimeout,
		TokenURL:     s.config.MembershipHookTokenURL,
		ClientID:     s.config.MembershipHookClientID,
		ClientSecret: s.config.MembershipHookClientSecret,
		Scopes:       strings.Fields(s.config.MembershipHookScope),
	})
	if err != nil {
		return err
	}
	s.app.MembershipHook = hook
	if s.app.AuditLogger == nil {
		al, err := command.NewAuditLogger(s.config.AuditLogPath)
		if err != nil {
			slog.Warn("failed to create audit logger, using stderr", "error", err)
			al, _ = command.NewAuditLogger("-")
		}
		s.app.AuditLogger = al
	}
	slog.Info("membership hook is on", "url", s.config.MembershipHookURL, "client_id", s.config.MembershipHookClientID)
	return nil
}

// blobStartTimeout bounds the wait for the object store at start.
const blobStartTimeout = 60 * time.Second

func (s *Server) initCommands() {
	assert.That(s.app != nil, "Server.initCommands: app not built")
	cueCfg, err := command.LoadCUE(s.config.CommandsCUEDir)
	if err != nil {
		slog.Warn("slash commands disabled: failed to load CUE definitions",
			"dir", s.config.CommandsCUEDir, "error", err)
		return
	}

	assert.That(cueCfg != nil, "Server.initCommands: nil CUE config without an error")
	reg := command.NewRegistry(cueCfg.Commands)
	s.app.CommandRegistry = reg

	// Register built-in handlers.
	handlers := make(map[string]command.Handler)
	handlers["help"] = helpCommand{reg: reg}
	handlers["invite"] = inviteCommand{app: s.app}
	handlers["kick"] = kickCommand{app: s.app}
	handlers["topic"] = topicCommand{app: s.app}

	assert.That(len(handlers) == 4, "Server.initCommands: a built-in handler is missing")
	s.app.CommandHandlers = handlers

	// Audit logger.
	al, err := command.NewAuditLogger(s.config.AuditLogPath)
	if err != nil {
		slog.Warn("failed to create audit logger, using stderr", "error", err)
		// The stderr logger opens no file and cannot fail; if it did, al
		// stays nil and commands are not audited.
		al, err = command.NewAuditLogger("-")
		errlog.Warn(err, "create stderr audit logger")
	}
	s.app.AuditLogger = al

	// Webhook channel. Config validation requires a URL when enabled.
	if s.config.WebhookEnabled {
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
	assert.That(s.store != nil, "Server.initJobs: store not open")
	assert.That(s.scheduler == nil, "Server.initJobs: scheduler already built")
	s.scheduler = jobs.NewScheduler()
	// Without ZincSearch there is nothing to index into, and an indexer
	// left running would fail and log every five seconds.
	if s.config.ZincSearchURL != "" {
		indexer := workers.NewSearchIndexer(
			s.config.ZincSearchURL,
			s.config.ZincSearchUser,
			s.config.ZincSearchPassword,
		).WithPostStore(s.store.Post())
		s.scheduler.AddWorker(indexer)
	}

	if s.app.Blobs != nil {
		s.scheduler.AddWorker(workers.NewFileSweeper(s.app.SweepFiles, s.config.FileSweepInterval))
	}

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
	assert.That(s.app != nil, "Server.initHTTP: app not built")
	handler := api.New(s.app)
	assert.That(handler != nil, "Server.initHTTP: nil handler")
	s.httpSrv = &http.Server{
		Addr:         s.config.ListenAddress,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// ensureMachineActors reconciles the declared non-human callers at boot.
//
// FATAL ON FAILURE, deliberately. These rows are the difference between a
// machine actor being authorized and being a 401, and a server that starts
// without them looks entirely healthy while refusing every client_credentials
// token — which is a much longer debugging session than a refusal to start.
func (s *Server) ensureMachineActors() error {
	assert.That(s.app != nil, "Server.ensureMachineActors: app not built")
	assert.That(s.config != nil, "Server.ensureMachineActors: nil config")
	actors, err := app.ParseMachineActors(s.config.MachineActors)
	if err != nil {
		return err
	}
	if err := app.CheckOnBehalfOfClients(s.config.OnBehalfOfClients, actors); err != nil {
		return err
	}
	if len(actors) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.app.EnsureMachineActors(ctx, actors)
}

// Start begins serving and blocks until a shutdown signal is received or the
// listener fails. Either way it shuts down every component before returning.
func (s *Server) Start() error {
	assert.That(s.scheduler != nil, "Server.Start: scheduler not built")
	assert.That(s.httpSrv != nil, "Server.Start: http server not built")
	s.scheduler.Start()

	slog.Info("server starting", "address", s.config.ListenAddress)

	errCh := make(chan error, 1)
	go func() {
		assert.That(cap(errCh) == 1, "Server.Start: the listener error must not block its send")
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		// The listener failed (a port in use, say). The scheduler, hub,
		// pubsub and store are already running and must still be stopped.
		return errors.Join(fmt.Errorf("server error: %w", err), s.Shutdown())
	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig)
	}

	return s.Shutdown()
}

// Shutdown gracefully shuts down all components.
func (s *Server) Shutdown() error {
	slog.Info("shutting down server")
	assert.That(s.httpSrv != nil && s.hub != nil && s.scheduler != nil, "Server.Shutdown: server not fully built")
	assert.That(s.app != nil && s.store != nil, "Server.Shutdown: app or store not built")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpSrv.Shutdown(ctx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}

	s.hub.Stop()
	s.scheduler.Stop()
	s.app.WaitBackground()

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
