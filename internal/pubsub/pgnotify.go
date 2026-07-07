package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// validTopicRe restricts topics to safe PostgreSQL channel identifiers.
// LISTEN cannot take bind parameters, so the topic is interpolated into the
// statement (quoted via pgx.Identifier) and must be validated first.
var validTopicRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

// waitInterval bounds WaitForNotification so the listener loop can pick up
// newly requested LISTEN topics; it caps subscription latency, not delivery
// latency (notifications interrupt the wait immediately).
const waitInterval = time.Second

// PGNotify implements PubSub using PostgreSQL LISTEN/NOTIFY.
//
// All LISTEN commands are executed on the single dedicated listener
// connection: LISTEN registers the *connection*, so subscribing on a pooled
// connection that is immediately released would never receive anything.
type PGNotify struct {
	pool     *pgxpool.Pool
	mu       sync.RWMutex
	handlers map[string][]func(data []byte)
	pending  chan string
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewPGNotify creates a new PGNotify backed by the given pgx pool.
func NewPGNotify(pool *pgxpool.Pool) (*PGNotify, error) {
	pg := &PGNotify{
		pool:     pool,
		handlers: make(map[string][]func(data []byte)),
		pending:  make(chan string, 16),
		done:     make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	pg.cancel = cancel

	go pg.listen(ctx)

	return pg, nil
}

func (pg *PGNotify) listen(ctx context.Context) {
	defer close(pg.done)

	for {
		if err := pg.listenOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("pgnotify: listener connection failed, reconnecting", "error", err)
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return
			}
		} else {
			return // clean shutdown
		}
	}
}

// listenOnce acquires a dedicated connection, re-establishes LISTEN for every
// known topic, and dispatches notifications until the connection fails or ctx
// is cancelled.
func (pg *PGNotify) listenOnce(ctx context.Context) error {
	conn, err := pg.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire: %w", err)
	}
	defer conn.Release()

	// Re-LISTEN all existing topics (fresh connection after a reconnect).
	pg.mu.RLock()
	topics := make([]string, 0, len(pg.handlers))
	for topic := range pg.handlers {
		topics = append(topics, topic)
	}
	pg.mu.RUnlock()
	for _, topic := range topics {
		if err := execListen(ctx, conn, topic); err != nil {
			return err
		}
	}

	for {
		// Pick up newly requested subscriptions.
		for {
			select {
			case topic := <-pg.pending:
				if err := execListen(ctx, conn, topic); err != nil {
					return err
				}
				continue
			default:
			}
			break
		}

		waitCtx, cancel := context.WithTimeout(ctx, waitInterval)
		notification, err := conn.Conn().WaitForNotification(waitCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			return fmt.Errorf("wait for notification: %w", err)
		}

		pg.mu.RLock()
		handlers := pg.handlers[notification.Channel]
		pg.mu.RUnlock()

		// Dispatch synchronously: handlers are expected to be fast, and this
		// keeps notification handling bounded (no goroutine-per-message).
		for _, handler := range handlers {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("pgnotify: handler panicked", "topic", notification.Channel, "panic", r)
					}
				}()
				handler([]byte(notification.Payload))
			}()
		}
	}
}

func execListen(ctx context.Context, conn *pgxpool.Conn, topic string) error {
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{topic}.Sanitize()); err != nil {
		return fmt.Errorf("listen %s: %w", topic, err)
	}
	slog.Info("pgnotify: listening", "topic", topic)
	return nil
}

// Publish sends data on the topic. The payload is passed through verbatim;
// note NOTIFY payloads are limited to ~8KB, so publish thin references and let
// subscribers fetch full records.
func (pg *PGNotify) Publish(ctx context.Context, topic string, data []byte) error {
	if !validTopicRe.MatchString(topic) {
		return fmt.Errorf("pgnotify publish: invalid topic %q", topic)
	}

	_, err := pg.pool.Exec(ctx, "SELECT pg_notify($1, $2)", topic, string(data))
	if err != nil {
		return fmt.Errorf("pgnotify publish: %w", err)
	}

	return nil
}

// Subscribe registers a handler for the topic. The LISTEN itself is executed
// by the listener goroutine on its dedicated connection.
func (pg *PGNotify) Subscribe(ctx context.Context, topic string, handler func(data []byte)) error {
	if !validTopicRe.MatchString(topic) {
		return fmt.Errorf("pgnotify subscribe: invalid topic %q", topic)
	}

	pg.mu.Lock()
	known := len(pg.handlers[topic]) > 0
	pg.handlers[topic] = append(pg.handlers[topic], handler)
	pg.mu.Unlock()

	if !known {
		select {
		case pg.pending <- topic:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	slog.Info("pgnotify: subscribed", "topic", topic)
	return nil
}

func (pg *PGNotify) Close() error {
	pg.cancel()
	<-pg.done
	return nil
}
