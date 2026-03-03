package mcp

import (
	"sync"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestEventBuffer_PushAndDrain(t *testing.T) {
	eb := NewEventBuffer()

	eb.Push(&model.WebSocketEvent{Event: "posted", Data: map[string]any{"msg": "a"}})
	eb.Push(&model.WebSocketEvent{Event: "posted", Data: map[string]any{"msg": "b"}})

	events := eb.Drain(0)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("unexpected sequences: %d, %d", events[0].Sequence, events[1].Sequence)
	}

	// Second drain should be empty since events were removed.
	events2 := eb.Drain(0)
	if len(events2) != 0 {
		t.Fatalf("expected 0 events on second drain, got %d", len(events2))
	}
}

func TestEventBuffer_DrainSinceSeq(t *testing.T) {
	eb := NewEventBuffer()

	eb.Push(&model.WebSocketEvent{Event: "a"})
	eb.Push(&model.WebSocketEvent{Event: "b"})
	eb.Push(&model.WebSocketEvent{Event: "c"})

	// Only events after seq 1
	events := eb.Drain(1)
	if len(events) != 2 {
		t.Fatalf("expected 2 events since seq 1, got %d", len(events))
	}
	if events[0].Event != "b" || events[1].Event != "c" {
		t.Fatalf("unexpected events: %s, %s", events[0].Event, events[1].Event)
	}
}

func TestEventBuffer_Overflow(t *testing.T) {
	eb := NewEventBuffer()

	// Push more than maxEventBufferSize (1000) events
	for i := 0; i < 1050; i++ {
		eb.Push(&model.WebSocketEvent{Event: "e"})
	}

	events := eb.Drain(0)
	if len(events) != 1000 {
		t.Fatalf("expected 1000 events after overflow, got %d", len(events))
	}
	// Oldest should have been evicted; first event should be seq 51
	if events[0].Sequence != 51 {
		t.Fatalf("expected first seq 51, got %d", events[0].Sequence)
	}
	if events[len(events)-1].Sequence != 1050 {
		t.Fatalf("expected last seq 1050, got %d", events[len(events)-1].Sequence)
	}
}

func TestEventBuffer_DrainEmpty(t *testing.T) {
	eb := NewEventBuffer()
	events := eb.Drain(0)
	if events != nil {
		t.Fatalf("expected nil from empty buffer, got %v", events)
	}
}

func TestEventBuffer_Concurrent(t *testing.T) {
	eb := NewEventBuffer()
	var wg sync.WaitGroup

	// 10 goroutines pushing 100 events each
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				eb.Push(&model.WebSocketEvent{Event: "concurrent"})
			}
		}()
	}

	// 5 goroutines draining concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				eb.Drain(0)
			}
		}()
	}

	wg.Wait()
	// If we get here without a race detector complaint, the test passes.
}
