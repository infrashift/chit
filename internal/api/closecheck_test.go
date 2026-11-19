package api

import (
	"io"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
)

// closeOK closes c and reports a failure as a test error. It is safe to call
// from any goroutine and from a deferred function or a t.Cleanup.
func closeOK(t *testing.T, c io.Closer, what string) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Errorf("close %s: %v", what, err)
	}
}

// readDeadlineOK sets conn's read deadline to d from now; a failure stops
// the test, so call it from the test goroutine.
func readDeadlineOK(t *testing.T, conn *gws.Conn, d time.Duration) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(d)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
}
