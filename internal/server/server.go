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
	s.hub = websocket.NewHub()
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

func (s *Server) initJobs() {
	s.scheduler = jobs.NewScheduler()
	indexer := workers.NewSearchIndexer(
		s.config.ZincSearchURL,
		s.config.ZincSearchUser,
		s.config.ZincSearchPassword,
	)
	s.scheduler.AddWorker(indexer)
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

	s.store.Close()

	slog.Info("server shutdown complete")
	return nil
}
