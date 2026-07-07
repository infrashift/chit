package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/infrashift/chit/internal/pubsub"
)

// fakePubSub delivers published messages synchronously to subscribers.
type fakePubSub struct {
	handlers map[string][]func(data []byte)
}

func newFakePubSub() *fakePubSub {
	return &fakePubSub{handlers: make(map[string][]func(data []byte))}
}

func (f *fakePubSub) Publish(_ context.Context, topic string, data []byte) error {
	for _, h := range f.handlers[topic] {
		h(data)
	}
	return nil
}

func (f *fakePubSub) Subscribe(_ context.Context, topic string, handler func(data []byte)) error {
	f.handlers[topic] = append(f.handlers[topic], handler)
	return nil
}

func (f *fakePubSub) Close() error { return nil }

func publishEnvelope(t *testing.T, ps *fakePubSub, env pubsub.EventEnvelope) {
	t.Helper()
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := ps.Publish(context.Background(), pubsub.TopicEvents, data); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

// The agent is a member of channelID (seeded in setupTestMCP fixtures) but not
// of other channels; targeted events for other users must also be filtered.
func TestEventFeed_FiltersAndBuffers(t *testing.T) {
	_, _, srv, cleanup := setupTestMCP(t)
	defer cleanup()

	ps := newFakePubSub()
	if err := srv.StartEventFeed(context.Background(), ps); err != nil {
		t.Fatalf("StartEventFeed: %v", err)
	}

	// Event in the agent's channel → buffered.
	publishEnvelope(t, ps, pubsub.EventEnvelope{Event: "posted", PostID: "p1", ChannelID: channelID})
	// Event in a channel the agent is not a member of → dropped.
	publishEnvelope(t, ps, pubsub.EventEnvelope{Event: "posted", PostID: "p2", ChannelID: "not-a-member"})
	// Targeted at another user → dropped.
	publishEnvelope(t, ps, pubsub.EventEnvelope{Event: "mentioned", PostID: "p3", TargetUserID: "someone-else"})
	// Targeted at the agent → buffered even without a membership check.
	publishEnvelope(t, ps, pubsub.EventEnvelope{Event: "mentioned", PostID: "p4", ChannelID: channelID, TargetUserID: agentUserID})

	events := srv.eventBuffer.Drain(0)
	if len(events) != 2 {
		t.Fatalf("expected 2 buffered events, got %d: %+v", len(events), events)
	}
	if events[0].Data["post_id"] != "p1" {
		t.Errorf("first event: got %v, want post_id=p1", events[0].Data)
	}
	if events[1].Event != "mentioned" || events[1].Data["post_id"] != "p4" {
		t.Errorf("second event: got %+v, want mentioned/p4", events[1])
	}
}
