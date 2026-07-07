package ws

import (
	"errors"
	"math"
	"time"
)

// ErrNotConnected is returned when an operation requires an active connection.
var ErrNotConnected = errors.New("ws: not connected")

// Backoff computes exponential backoff durations with a maximum cap.
type Backoff struct {
	attempt int
	Base    time.Duration
	Max     time.Duration
}

// NewBackoff creates a Backoff with sensible defaults.
func NewBackoff() *Backoff {
	return &Backoff{
		Base: 500 * time.Millisecond,
		Max:  30 * time.Second,
	}
}

// Next returns the duration to wait before the next attempt and increments the counter.
func (b *Backoff) Next() time.Duration {
	d := time.Duration(float64(b.Base) * math.Pow(2, float64(b.attempt)))
	if d > b.Max {
		d = b.Max
	}
	b.attempt++
	return d
}

// Reset clears the attempt counter after a successful connection.
func (b *Backoff) Reset() {
	b.attempt = 0
}
