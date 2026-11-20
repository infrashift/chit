package pubsub

import "context"

// PubSub abstracts cross-node event distribution.
type PubSub interface {
	Publish(ctx context.Context, topic string, data []byte) error
	Subscribe(ctx context.Context, topic string, handler Handler) error
	Close() error
}

// Handler receives the messages published on a subscribed topic.
type Handler interface {
	Handle(data []byte)
}
