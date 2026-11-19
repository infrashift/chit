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

	"github.com/infrashift/chit/internal/assert"
)

// validTopicRe restricts topics to safe PostgreSQL channel identifiers.
// LISTEN cannot take bind parameters, so the topic is interpolated into the
// statement (quoted via pgx.Identifier) and must be validated first.
//
// semscope: allow(NASA-P10-06) compiled once, never changed
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
	handlers map[string][]Handler
	pending  chan string
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewPGNotify creates a new PGNotify backed by the given pgx pool.
func NewPGNotify(pool *pgxpool.Pool) (*PGNotify, error) {
	assert.That(pool != nil, "NewPGNotify: nil pool")
	pg := &PGNotify{
		pool:     pool,
		handlers: make(map[string][]Handler),
		pending:  make(chan string, 16),
		done:     make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	pg.cancel = cancel
	assert.That(cap(pg.pending) > 0, "NewPGNotify: unbuffered pending channel")

	go pg.listen(ctx)

	return pg, nil
}

func (pg *PGNotify) listen(ctx context.Context) {
	assert.That(pg.done != nil, "PGNotify.listen: nil done channel")
	assert.That(pg.pool != nil, "PGNotify.listen: nil pool")
	defer close(pg.done)

	for ctx.Err() == nil {
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
	assert.That(conn != nil, "PGNotify.listenOnce: nil connection without an error")
	defer conn.Release()

	if err := pg.relistenAll(ctx, conn); err != nil {
		return err
	}

	for ctx.Err() == nil {
		if err := pg.listenPending(ctx, conn); err != nil {
			return err
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

		assert.That(notification != nil, "PGNotify.listenOnce: nil notification without an error")
		pg.dispatch(notification.Channel, []byte(notification.Payload))
	}
	return nil
}

// relistenAll re-LISTENs all existing topics (fresh connection after a
// reconnect).
func (pg *PGNotify) relistenAll(ctx context.Context, conn *pgxpool.Conn) error {
	assert.That(conn != nil, "PGNotify.relistenAll: nil connection")
	pg.mu.RLock()
	topics := make([]string, 0, len(pg.handlers))
	for topic := range pg.handlers {
		topics = append(topics, topic)
	}
	known := len(pg.handlers)
	pg.mu.RUnlock()
	assert.That(len(topics) == known, "PGNotify.relistenAll: topic list does not match the handler map")
	for _, topic := range topics {
		if err := execListen(ctx, conn, topic); err != nil {
			return err
		}
	}
	return nil
}

// listenPending picks up newly requested subscriptions. This goroutine is the
// only receiver, so a receive after len > 0 does not block, and the capacity
// bounds the loop (Power of 10 rule 2).
func (pg *PGNotify) listenPending(ctx context.Context, conn *pgxpool.Conn) error {
	assert.That(conn != nil, "PGNotify.listenPending: nil connection")
	assert.That(pg.pending != nil, "PGNotify.listenPending: nil pending channel")
	for range cap(pg.pending) {
		if len(pg.pending) == 0 {
			break
		}
		if err := execListen(ctx, conn, <-pg.pending); err != nil {
			return err
		}
	}
	return nil
}

// dispatch hands a notification's payload to every handler of its topic.
// Dispatch is synchronous: handlers are expected to be fast, and this keeps
// notification handling bounded (no goroutine-per-message).
func (pg *PGNotify) dispatch(topic string, payload []byte) {
	pg.mu.RLock()
	assert.That(pg.handlers != nil, "PGNotify.dispatch: nil handler map")
	handlers := pg.handlers[topic]
	pg.mu.RUnlock()

	for _, handler := range handlers {
		assert.That(handler != nil, "PGNotify.dispatch: nil handler registered")
		handleRecovered(topic, handler, payload)
	}
}

// handleRecovered runs one handler, logging rather than propagating a panic.
func handleRecovered(topic string, handler Handler, payload []byte) {
	assert.That(handler != nil, "handleRecovered: nil handler")
	assert.That(topic != "", "handleRecovered: empty topic")
	defer func() {
		if r := recover(); r != nil {
			slog.Error("pgnotify: handler panicked", "topic", topic, "panic", r)
		}
	}()
	handler.Handle(payload)
}

func execListen(ctx context.Context, conn *pgxpool.Conn, topic string) error {
	assert.That(conn != nil, "execListen: nil connection")
	assert.That(validTopicRe.MatchString(topic), "execListen: topic was not validated by Subscribe")
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
	assert.That(pg.pool != nil, "PGNotify.Publish: nil pool")

	_, err := pg.pool.Exec(ctx, "SELECT pg_notify($1, $2)", topic, string(data))
	if err != nil {
		return fmt.Errorf("pgnotify publish: %w", err)
	}

	return nil
}

// Subscribe registers a handler for the topic. The LISTEN itself is executed
// by the listener goroutine on its dedicated connection.
func (pg *PGNotify) Subscribe(ctx context.Context, topic string, handler Handler) error {
	if !validTopicRe.MatchString(topic) {
		return fmt.Errorf("pgnotify subscribe: invalid topic %q", topic)
	}
	assert.That(handler != nil, "PGNotify.Subscribe: nil handler")
	assert.That(pg.pending != nil, "PGNotify.Subscribe: nil pending channel")

	pg.mu.Lock()
	assert.That(pg.handlers != nil, "PGNotify.Subscribe: nil handler map")
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
	assert.That(pg.cancel != nil, "PGNotify.Close: nil cancel func")
	assert.That(pg.done != nil, "PGNotify.Close: nil done channel")
	pg.cancel()
	<-pg.done
	return nil
}
