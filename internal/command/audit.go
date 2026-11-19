package command

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/infrashift/chit/internal/assert"
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
		assert.That(l != nil, "NewAuditLogger: nil stderr logger")
		return &AuditLogger{logger: l}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}

	assert.That(f != nil, "NewAuditLogger: nil file without an error")
	l := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{}))
	return &AuditLogger{logger: l, file: f}, nil
}

// LogCommandAttempt records an audit entry for a slash command invocation.
func (a *AuditLogger) LogCommandAttempt(eventID, actorID, actorType, commandSlug string, authorized bool) {
	assert.That(a != nil && a.logger != nil, "AuditLogger.LogCommandAttempt: logger not built by NewAuditLogger")
	assert.That(eventID != "" && commandSlug != "", "AuditLogger.LogCommandAttempt: empty event id or slug")
	a.logger.Info("command_attempt",
		"event_id", eventID,
		"actor_id", actorID,
		"actor_type", actorType,
		"command_slug", commandSlug,
		"authorized", authorized,
	)
}

// LogPostOnBehalfOf records a post that an allowlisted OAuth2 client made on
// behalf of a person: the actor (the client's user and client ID) and the
// principal (the person, who is the post's author).
func (a *AuditLogger) LogPostOnBehalfOf(eventID, actorID, clientID, principalID, postID, channelID string) {
	assert.That(a != nil && a.logger != nil, "AuditLogger.LogPostOnBehalfOf: logger not built by NewAuditLogger")
	assert.That(eventID != "" && actorID != "", "AuditLogger.LogPostOnBehalfOf: empty event or actor id")
	assert.That(principalID != "" && postID != "", "AuditLogger.LogPostOnBehalfOf: empty principal or post id")
	a.logger.Info("post_on_behalf_of",
		"event_id", eventID,
		"actor_id", actorID,
		"client_id", clientID,
		"principal_id", principalID,
		"post_id", postID,
		"channel_id", channelID,
	)
}

// LogOnBehalfOf records an act other than a post that an allowlisted OAuth2
// client did on behalf of a person: action names it (user_provisioned,
// channel_created, channel_member_added), and attrs are its key-value pairs.
func (a *AuditLogger) LogOnBehalfOf(eventID, action, actorID, clientID, principalID string, attrs ...any) {
	assert.That(a != nil && a.logger != nil, "AuditLogger.LogOnBehalfOf: logger not built by NewAuditLogger")
	assert.That(eventID != "" && action != "", "AuditLogger.LogOnBehalfOf: empty event id or action")
	assert.That(len(attrs)%2 == 0, "AuditLogger.LogOnBehalfOf: attrs are not key-value pairs")
	a.logger.Info("on_behalf_of", append([]any{
		"event_id", eventID,
		"action", action,
		"actor_id", actorID,
		"client_id", clientID,
		"principal_id", principalID,
	}, attrs...)...)
}

// Close flushes and closes the underlying file, if any.
func (a *AuditLogger) Close() error {
	assert.That(a != nil, "AuditLogger.Close: nil logger")
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}
