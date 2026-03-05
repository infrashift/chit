package command

import (
	"log/slog"
	"os"
	"path/filepath"
)

// AuditLogger writes structured JSON audit records for command executions.
type AuditLogger struct {
	logger *slog.Logger
	file   *os.File // nil when writing to stderr
}

// NewAuditLogger creates an AuditLogger that appends JSON lines to the given
// path. If path is empty or "-", it writes to stderr (useful in development).
func NewAuditLogger(path string) (*AuditLogger, error) {
	if path == "" || path == "-" {
		l := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{}))
		return &AuditLogger{logger: l}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}

	l := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{}))
	return &AuditLogger{logger: l, file: f}, nil
}

// LogCommandAttempt records an audit entry for a slash command invocation.
func (a *AuditLogger) LogCommandAttempt(eventID, actorID, actorType, commandSlug string, authorized bool) {
	a.logger.Info("command_attempt",
		"event_id", eventID,
		"actor_id", actorID,
		"actor_type", actorType,
		"command_slug", commandSlug,
		"authorized", authorized,
	)
}

// Close flushes and closes the underlying file, if any.
func (a *AuditLogger) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}
