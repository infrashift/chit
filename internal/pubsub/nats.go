package pubsub

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

// NatsPubSub implements PubSub using NATS.
type NatsPubSub struct {
	conn *nats.Conn
	subs []*nats.Subscription
}

// NewNatsPubSub connects to a NATS server and returns a PubSub implementation.
func NewNatsPubSub(url string) (*NatsPubSub, error) {
	nc, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("nats reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	slog.Info("connected to nats", "url", url)
	return &NatsPubSub{conn: nc}, nil
}

func (n *NatsPubSub) Publish(_ context.Context, topic string, data []byte) error {
	if err := n.conn.Publish(topic, data); err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}
	return nil
}

func (n *NatsPubSub) Subscribe(_ context.Context, topic string, handler func(data []byte)) error {
	sub, err := n.conn.Subscribe(topic, func(msg *nats.Msg) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("nats: handler panicked", "topic", topic, "panic", r)
			}
		}()
		handler(msg.Data)
	})
	if err != nil {
		return fmt.Errorf("nats subscribe: %w", err)
	}

	n.subs = append(n.subs, sub)
	slog.Info("nats: subscribed", "topic", topic)
	return nil
}

func (n *NatsPubSub) Close() error {
	for _, sub := range n.subs {
		if err := sub.Unsubscribe(); err != nil {
			slog.Warn("nats unsubscribe error", "error", err)
		}
	}
	n.conn.Close()
	return nil
}
