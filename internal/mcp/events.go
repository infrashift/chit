package mcp

import (
	"sync"

	"github.com/infrashift/chit/internal/assert"
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
	eb := &EventBuffer{
		events: make([]*model.WebSocketEvent, 0, 128),
	}
	return eb
}

// Push adds an event to the buffer, evicting the oldest if at capacity.
func (eb *EventBuffer) Push(event *model.WebSocketEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	assert.That(event != nil, "EventBuffer.Push: nil event")
	assert.That(len(eb.events) == 0 || eb.events[len(eb.events)-1].Sequence == eb.seq,
		"EventBuffer.Push: newest buffered event is not the last pushed")
	eb.seq++
	event.Sequence = eb.seq

	if len(eb.events) >= maxEventBufferSize {
		eb.events = eb.events[1:]
	}
	eb.events = append(eb.events, event)
	assert.That(len(eb.events) <= maxEventBufferSize, "EventBuffer.Push: buffer above its bound")
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

	assert.That(len(result) <= len(eb.events), "EventBuffer.Drain: more results than buffered events")

	// Remove drained events
	if len(result) > 0 {
		lastSeq := result[len(result)-1].Sequence
		assert.That(lastSeq == eb.seq, "EventBuffer.Drain: newest result is not the last pushed event")
		remaining := eb.events[:0]
		for _, e := range eb.events {
			if e.Sequence > lastSeq {
				remaining = append(remaining, e)
			}
		}
		eb.events = remaining
		assert.That(len(eb.events) == 0, "EventBuffer.Drain: events newer than the newest one remain")
	}

	return result
}
