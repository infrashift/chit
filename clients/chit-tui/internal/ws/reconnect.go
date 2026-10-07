package ws

import (
	"errors"
	"math"
	"math/rand/v2"
	"time"
)

// ErrNotConnected is returned when an operation requires an active connection.
var ErrNotConnected = errors.New("ws: not connected")

// ErrUnauthorized marks a handshake the server rejected for its credentials.
// Redialing cannot fix it; the user has to sign in again.
var ErrUnauthorized = errors.New("unauthorized")

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

// Next returns the duration to wait before the next attempt and increments
// the counter. Up to a fifth of the delay is shaved off at random, so clients
// dropped by the same server restart do not all redial in the same instant.
func (b *Backoff) Next() time.Duration {
	d := b.Max
	// Compare before converting: past about 2^35 the product no longer fits
	// a Duration and wraps negative, which turned a long outage into a busy
	// dial loop.
	if f := float64(b.Base) * math.Pow(2, float64(b.attempt)); f < float64(b.Max) {
		d = time.Duration(f)
		b.attempt++
	}
	return d - time.Duration(rand.Int64N(int64(d)/5+1))
}

// Reset clears the attempt counter after a successful connection.
func (b *Backoff) Reset() {
	b.attempt = 0
}
