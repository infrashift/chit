package app

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
)

// publishEvent delivers the event to local WebSocket clients via the hub and
// publishes a thin envelope to pubsub for out-of-process consumers. Nothing in
// this repository subscribes: chit-mcp and the bridge read chitd's WebSocket.
// chitd itself does not re-consume the topic, so WebSocket fan-out remains
// single-node.
func (a *App) publishEvent(ctx context.Context, event *model.WebSocketEvent, env *pubsub.EventEnvelope) {
	a.Hub.Broadcast(event)

	if a.PubSub == nil {
		return
	}
	data, err := json.Marshal(env)
	if err != nil {
		slog.Error("failed to marshal event envelope", "event", env.Event, "error", err)
		return
	}
	if err := a.PubSub.Publish(ctx, pubsub.TopicEvents, data); err != nil {
		slog.Warn("failed to publish event", "event", env.Event, "error", err)
	}
}
