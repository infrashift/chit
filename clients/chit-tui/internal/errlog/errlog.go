// Package errlog checks the errors that a caller cannot return and cannot
// act on, for NASA Power of 10 rule 7 (ADR-0002 and ADR-0010 in
// .okfw/decisions). A deferred Close and a best-effort cleanup are examples.
// errlog logs each such error with slog, so no error goes to the blank
// identifier.
package errlog

import (
	"io"
	"log/slog"
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
