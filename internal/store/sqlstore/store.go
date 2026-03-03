package sqlstore

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/infrashift/chit/internal/store"
)

// SqlStore implements store.Store backed by PostgreSQL via pgx.
type SqlStore struct {
	pool    *pgxpool.Pool
	user    store.UserStore
	team    store.TeamStore
	channel store.ChannelStore
	post    store.PostStore
	thread  store.ThreadStore
	tag     store.TagStore
}

// New creates a new SqlStore and connects to the database.
func New(ctx context.Context, databaseURL string, maxOpen, maxIdle int) (*SqlStore, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	cfg.MaxConns = int32(maxOpen)
	cfg.MinConns = int32(maxIdle)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	slog.Info("connected to database")

	ss := &SqlStore{pool: pool}
	ss.user = &SqlUserStore{sqlStore: ss}
	ss.team = &SqlTeamStore{sqlStore: ss}
	ss.channel = &SqlChannelStore{sqlStore: ss}
	ss.post = &SqlPostStore{sqlStore: ss}
	ss.thread = &SqlThreadStore{sqlStore: ss}
	ss.tag = &SqlTagStore{sqlStore: ss}

	return ss, nil
}

func (ss *SqlStore) User() store.UserStore       { return ss.user }
func (ss *SqlStore) Team() store.TeamStore       { return ss.team }
func (ss *SqlStore) Channel() store.ChannelStore { return ss.channel }
func (ss *SqlStore) Post() store.PostStore       { return ss.post }
func (ss *SqlStore) Thread() store.ThreadStore   { return ss.thread }
func (ss *SqlStore) Tag() store.TagStore         { return ss.tag }

func (ss *SqlStore) Close() {
	ss.pool.Close()
	slog.Info("database connection closed")
}

// Pool returns the underlying pgxpool for use by sub-stores.
func (ss *SqlStore) Pool() *pgxpool.Pool {
	return ss.pool
}
