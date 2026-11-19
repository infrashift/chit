package errlog

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type closer struct{ err error }

func (c closer) Close() error { return c.err }

// capture sends the slog output at the debug level to a buffer for the test.
func capture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestCloseLogsOnlyAFailure(t *testing.T) {
	buf := capture(t)
	Close(closer{}, "good")
	if buf.Len() != 0 {
		t.Fatalf("a good Close logged %q", buf.String())
	}
	Close(closer{err: errors.New("boom")}, "bad body")
	if got := buf.String(); !strings.Contains(got, "bad body") || !strings.Contains(got, "boom") {
		t.Fatalf("log = %q, want the resource and the error", got)
	}
}

func TestDebugAndWarnLogOnlyAnError(t *testing.T) {
	buf := capture(t)
	Debug(nil, "nil debug")
	Warn(nil, "nil warn")
	if buf.Len() != 0 {
		t.Fatalf("a nil error logged %q", buf.String())
	}
	Debug(errors.New("d"), "debug what")
	Warn(errors.New("w"), "warn what")
	got := buf.String()
	if !strings.Contains(got, "level=DEBUG msg=\"debug what\"") || !strings.Contains(got, "level=WARN msg=\"warn what\"") {
		t.Fatalf("log = %q, want one debug and one warn record", got)
	}
}
