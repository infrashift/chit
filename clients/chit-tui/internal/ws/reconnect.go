package ws

import (
	"errors"
	"math"
	"math/rand/v2"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// ErrNotConnected is returned when an operation requires an active connection.
//
// semscope: allow(NASA-P10-06) sentinel error: errors.Is needs one identity
var ErrNotConnected = errors.New("ws: not connected")

// ErrUnauthorized marks a handshake the server rejected for its credentials.
// Redialing cannot fix it; the user has to sign in again.
//
// semscope: allow(NASA-P10-06) sentinel error: errors.Is needs one identity
var ErrUnauthorized = errors.New("unauthorized")

// Backoff computes exponential backoff durations with a maximum cap.
type Backoff struct {
	attempt int
	Base    time.Duration
	Max     time.Duration
}

// The default redial schedule: the first retry comes after about half a
// second, and no wait is longer than half a minute.
const (
	defaultBackoffBase = 500 * time.Millisecond
	defaultBackoffMax  = 30 * time.Second
)

// NewBackoff creates a Backoff with sensible defaults.
func NewBackoff() *Backoff {
	b := &Backoff{
		Base: defaultBackoffBase,
		Max:  defaultBackoffMax,
	}
	assert.That(b.Base > 0 && b.Max >= b.Base, "NewBackoff: the default schedule must start positive and stay under its cap")
	return b
}

// Next returns the duration to wait before the next attempt and increments
// the counter. Up to a fifth of the delay is shaved off at random, so clients
// dropped by the same server restart do not all redial in the same instant.
func (b *Backoff) Next() time.Duration {
	assert.That(b.Base > 0, "Backoff.Next: base must be positive")
	assert.That(b.Max >= b.Base, "Backoff.Next: max is below base")
	assert.That(b.attempt >= 0, "Backoff.Next: negative attempt count")
	d := b.Max
	// Compare before converting: past about 2^35 the product no longer fits
	// a Duration and wraps negative, which turned a long outage into a busy
	// dial loop.
	if f := float64(b.Base) * math.Pow(2, float64(b.attempt)); f < float64(b.Max) {
		d = time.Duration(f)
		b.attempt++
		// The counter stops once the delay reaches Max, so it stays small
		// enough for the power of two above to be exact.
		assert.That(b.attempt < 64, "Backoff.Next: attempt counter grew without bound")
	}
	assert.That(d > 0 && d <= b.Max, "Backoff.Next: delay outside (0, max]")
	wait := d - time.Duration(rand.Int64N(int64(d)/5+1))
	assert.That(wait > 0 && wait <= d, "Backoff.Next: jitter left the delay outside (0, d]")
	return wait
}

// Reset clears the attempt counter after a successful connection.
func (b *Backoff) Reset() {
	assert.That(b != nil, "Backoff.Reset: nil backoff")
	b.attempt = 0
}
