package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PGNotify implements PubSub using PostgreSQL LISTEN/NOTIFY.
type PGNotify struct {
	pool     *pgxpool.Pool
	mu       sync.RWMutex
	handlers map[string][]func(data []byte)
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewPGNotify creates a new PGNotify backed by the given pgx pool.
func NewPGNotify(pool *pgxpool.Pool) (*PGNotify, error) {
	pg := &PGNotify{
		pool:     pool,
		handlers: make(map[string][]func(data []byte)),
		done:     make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	pg.cancel = cancel

	go pg.listen(ctx)

	return pg, nil
}

func (pg *PGNotify) listen(ctx context.Context) {
	defer close(pg.done)

	conn, err := pg.pool.Acquire(ctx)
	if err != nil {
		slog.Error("pgnotify: failed to acquire connection", "error", err)
		return
	}
	defer conn.Release()

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("pgnotify: error waiting for notification", "error", err)
			return
		}

		pg.mu.RLock()
		handlers := pg.handlers[notification.Channel]
		pg.mu.RUnlock()

		for _, handler := range handlers {
			h := handler
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("pgnotify: handler panicked", "topic", notification.Channel, "panic", r)
					}
				}()
				h([]byte(notification.Payload))
			}()
		}
	}
}

func (pg *PGNotify) Publish(ctx context.Context, topic string, data []byte) error {
	payload, err := json.Marshal(string(data))
	if err != nil {
		return fmt.Errorf("pgnotify publish marshal: %w", err)
	}

	_, err = pg.pool.Exec(ctx, "SELECT pg_notify($1, $2)", topic, string(payload))
	if err != nil {
		return fmt.Errorf("pgnotify publish: %w", err)
	}

	return nil
}

func (pg *PGNotify) Subscribe(ctx context.Context, topic string, handler func(data []byte)) error {
	conn, err := pg.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("pgnotify subscribe acquire: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, fmt.Sprintf("LISTEN %s", topic))
	if err != nil {
		return fmt.Errorf("pgnotify subscribe listen: %w", err)
	}

	pg.mu.Lock()
	pg.handlers[topic] = append(pg.handlers[topic], handler)
	pg.mu.Unlock()

	slog.Info("pgnotify: subscribed", "topic", topic)
	return nil
}

func (pg *PGNotify) Close() error {
	pg.cancel()
	<-pg.done
	return nil
}
