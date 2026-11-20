package sqlstore

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/infrashift/chit/internal/assert"
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
	file    store.FileStore
}

// New creates a new SqlStore and connects to the database.
func New(ctx context.Context, databaseURL string, maxOpen, maxIdle int) (*SqlStore, error) {
	assert.That(databaseURL != "", "sqlstore.New: empty database URL")
	assert.That(maxOpen > 0, "sqlstore.New: maxOpen must be positive")
	assert.That(maxIdle >= 0 && maxIdle <= maxOpen, "sqlstore.New: maxIdle out of [0, maxOpen]")
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
	assert.That(ss.pool != nil, "sqlstore.New: nil pool after connect")
	ss.file = &SqlFileStore{sqlStore: ss}

	return ss, nil
}

func (ss *SqlStore) User() store.UserStore {
	assert.That(ss.user != nil, "SqlStore.User: nil sub-store")
	return ss.user
}

func (ss *SqlStore) Team() store.TeamStore {
	assert.That(ss.team != nil, "SqlStore.Team: nil sub-store")
	return ss.team
}

func (ss *SqlStore) Channel() store.ChannelStore {
	assert.That(ss.channel != nil, "SqlStore.Channel: nil sub-store")
	return ss.channel
}

func (ss *SqlStore) Post() store.PostStore {
	assert.That(ss.post != nil, "SqlStore.Post: nil sub-store")
	return ss.post
}

func (ss *SqlStore) Thread() store.ThreadStore {
	assert.That(ss.thread != nil, "SqlStore.Thread: nil sub-store")
	return ss.thread
}

func (ss *SqlStore) Tag() store.TagStore {
	assert.That(ss.tag != nil, "SqlStore.Tag: nil sub-store")
	return ss.tag
}

func (ss *SqlStore) File() store.FileStore {
	assert.That(ss.file != nil, "SqlStore.File: nil sub-store")
	return ss.file
}

func (ss *SqlStore) Close() {
	assert.That(ss.pool != nil, "SqlStore.Close: nil pool")
	ss.pool.Close()
	slog.Info("database connection closed")
}

// Pool returns the underlying pgxpool for use by sub-stores.
func (ss *SqlStore) Pool() *pgxpool.Pool {
	assert.That(ss.pool != nil, "SqlStore.Pool: nil pool")
	return ss.pool
}
