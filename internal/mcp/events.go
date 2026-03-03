package mcp

import (
	"sync"

	"github.com/infrashift/chit/internal/model"
)

const maxEventBufferSize = 1000

// EventBuffer is a mutex-protected ring buffer that stores WebSocket events
// for MCP agents. Agents poll events via the get_new_events tool rather than
// maintaining a persistent WebSocket connection.
type EventBuffer struct {
	mu     sync.Mutex
	events []*model.WebSocketEvent
	seq    int64
}

// NewEventBuffer creates an empty event buffer.
func NewEventBuffer() *EventBuffer {
	return &EventBuffer{
		events: make([]*model.WebSocketEvent, 0, 128),
	}
}

// Push adds an event to the buffer, evicting the oldest if at capacity.
func (eb *EventBuffer) Push(event *model.WebSocketEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.seq++
	event.Sequence = eb.seq

	if len(eb.events) >= maxEventBufferSize {
		eb.events = eb.events[1:]
	}
	eb.events = append(eb.events, event)
}

// Drain returns and removes all buffered events since the given sequence number.
// Pass 0 to get all buffered events.
func (eb *EventBuffer) Drain(sinceSeq int64) []*model.WebSocketEvent {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	var result []*model.WebSocketEvent
	for _, e := range eb.events {
		if e.Sequence > sinceSeq {
			result = append(result, e)
		}
	}

	// Remove drained events
	if len(result) > 0 {
		lastSeq := result[len(result)-1].Sequence
		remaining := eb.events[:0]
		for _, e := range eb.events {
			if e.Sequence > lastSeq {
				remaining = append(remaining, e)
			}
		}
		eb.events = remaining
	}

	return result
}
