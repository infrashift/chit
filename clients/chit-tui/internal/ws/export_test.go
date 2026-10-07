package ws

import "time"

// SetTimings shortens the client's reconnect and liveness timers so tests do
// not have to wait out production intervals. Call it before Connect.
func SetTimings(c WSClient, backoffBase, pongWait, pingPeriod time.Duration) {
	wc := c.(*wsClient)
	wc.newBackoff = func() *Backoff {
		b := NewBackoff()
		b.Base = backoffBase
		b.Max = 4 * backoffBase
		return b
	}
	wc.pongWait = pongWait
	wc.pingPeriod = pingPeriod
}
