package ws

import (
	"fmt"
	"time"
)

// SetTimings shortens the client's reconnect and liveness timers so tests do
// not have to wait out production intervals. Call it before Connect.
func SetTimings(c WSClient, backoffBase, pongWait, pingPeriod time.Duration) {
	wc, ok := c.(*wsClient)
	if !ok {
		panic(fmt.Sprintf("ws.SetTimings: %T is not a client from this package", c))
	}
	wc.backoffBase = backoffBase
	wc.backoffMax = 4 * backoffBase
	wc.pongWait = pongWait
	wc.pingPeriod = pingPeriod
}
