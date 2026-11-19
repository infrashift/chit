// Package errlog checks the errors that a caller cannot return and cannot
// act on, for NASA Power of 10 rule 7 (ADR-0002 and ADR-0010 in
// .okfw/decisions). A deferred Close and a best-effort cleanup are examples.
// errlog logs each such error with slog, so no error goes to the blank
// identifier.
package errlog

import (
	"io"
	"log/slog"
	"runtime/debug"
)

// Close closes c and logs a failure at the debug level. what names the
// resource, for example "keto response body". Use it in a defer statement.
func Close(c io.Closer, what string) {
	if err := c.Close(); err != nil {
		slog.Debug("close failed", "what", what, "err", err)
	}
}

// Debug logs err at the debug level when it is not nil. Use it for an error
// that is expected and harmless, for example a write to a peer that left.
func Debug(err error, what string) {
	if err != nil {
		slog.Debug(what, "err", err)
	}
}

// Warn logs err at the warning level when it is not nil. Use it for an error
// that an operator must see but that the caller cannot return.
func Warn(err error, what string) {
	if err != nil {
		slog.Warn(what, "err", err)
	}
}

// Recover stops a panic in a goroutine that serves one unit of work, for
// example one WebSocket connection or one thread, and logs the panic with
// its stack. The goroutine then ends, and the process goes on. Defer it as
// the first statement of the goroutine, so the other deferred calls run
// before it. A loop that the whole process needs does not use Recover: a
// failed assertion there stops the process, and the supervisor restarts it
// (ADR-0011).
func Recover(what string) {
	if r := recover(); r != nil {
		slog.Error("goroutine stopped after a panic", "what", what, "panic", r, "stack", string(debug.Stack()))
	}
}
