package pubsub

import "context"

// PubSub abstracts cross-node event distribution.
type PubSub interface {
	Publish(ctx context.Context, topic string, data []byte) error
	Subscribe(ctx context.Context, topic string, handler func(data []byte)) error
	Close() error
}
