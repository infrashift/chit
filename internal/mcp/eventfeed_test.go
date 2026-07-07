package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/chitclient"
	"github.com/infrashift/chit/internal/model"
)

// The feed consumes chitd's WebSocket (which already filters events
// server-side); the only client-side rule left is dropping ephemeral
// command_response events.
func TestEventFeed_BuffersWSEvents(t *testing.T) {
	fake := newFakeChit(t)
	client := chitclient.New(fake.server.URL, agentKratosID, "")
	srv := New(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.StartEventFeed(ctx)

	fake.pushEvent(&model.WebSocketEvent{
		Event: model.WebSocketEventPosted,
		Data:  map[string]any{"id": "p1", "content": "full post payload"},
	})
	fake.pushEvent(&model.WebSocketEvent{
		Event: model.WebSocketEventCommandResponse,
		Data:  map[string]any{"text": "ephemeral"},
	})
	fake.pushEvent(&model.WebSocketEvent{
		Event: model.WebSocketEventMentioned,
		Data:  map[string]any{"post_id": "p4"},
	})

	// The feed is asynchronous (WS connect + reads); poll until the two
	// non-ephemeral events arrive.
	deadline := time.After(5 * time.Second)
	var events []*model.WebSocketEvent
	for len(events) < 2 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for events, got %d: %+v", len(events), events)
		case <-time.After(10 * time.Millisecond):
			events = append(events, srv.eventBuffer.Drain(0)...)
		}
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 buffered events, got %d: %+v", len(events), events)
	}
	if events[0].Event != model.WebSocketEventPosted || events[0].Data["id"] != "p1" {
		t.Errorf("first event: got %+v, want posted/p1", events[0])
	}
	if events[1].Event != model.WebSocketEventMentioned || events[1].Data["post_id"] != "p4" {
		t.Errorf("second event: got %+v, want mentioned/p4", events[1])
	}

	// command_response must never surface.
	for _, e := range events {
		if e.Event == model.WebSocketEventCommandResponse {
			t.Error("command_response leaked into the event buffer")
		}
	}
}
