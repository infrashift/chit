package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// UserMessage returns a short text for a person about err. It names the
// cause for the errors that the server marks with a stable ID or status,
// and it returns err.Error() for each other error.
func UserMessage(err error) string {
	assert.That(err != nil, "UserMessage: nil error")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}
	assert.That(apiErr != nil, "UserMessage: errors.As matched a nil *APIError")
	if msg, ok := messageForID(apiErr); ok {
		return msg
	}
	if msg, ok := messageForStatus(apiErr); ok {
		return msg
	}
	return apiErr.Error()
}

// messageForID gives the text for an error that the server marks with a
// stable ID.
func messageForID(e *APIError) (string, bool) {
	assert.That(e != nil, "messageForID: nil error")
	id := e.ID()
	switch {
	case id == "channel.membership_refused":
		return refusalMessage(e), true
	case id == "file.disabled":
		return "attachments are off on this server", true
	case id == "file.too_large":
		return "the file is too large (10 MB at most)", true
	case id == "file.quota_exceeded":
		return "your attachment quota is full", true
	case id == "file.svg", id == "file.unsupported_type", id == "file.type_mismatch":
		return "the server accepts only PNG, JPEG, WebP and GIF images", true
	case strings.HasPrefix(id, "file."):
		return "the server refused the image: " + serverText(e), true
	}
	return "", false
}

// messageForStatus gives the text for a status that has one meaning for
// the person, whatever the route.
func messageForStatus(e *APIError) (string, bool) {
	assert.That(e != nil, "messageForStatus: nil error")
	switch e.StatusCode {
	case http.StatusTooManyRequests:
		if e.RetryAfter > 0 {
			return fmt.Sprintf("too many requests: try again in %ds", int(e.RetryAfter.Seconds()+0.5)), true
		}
		return "too many requests: try again soon", true
	case http.StatusConflict:
		return "that name is in use: " + serverText(e), true
	}
	return "", false
}

// refusalMessage tells why the membership policy refused an add.
func refusalMessage(e *APIError) string {
	assert.That(e != nil && e.AppError != nil, "refusalMessage: no server error")
	if e.AppError.Detail != "" {
		return "the membership policy refused this: " + e.AppError.Detail
	}
	if e.AppError.Reason != "" {
		return "the membership policy refused this (" + e.AppError.Reason + ")"
	}
	return "the membership policy refused this"
}

// serverText returns the server's own message, or the status text when the
// body had none.
func serverText(e *APIError) string {
	assert.That(e != nil, "serverText: nil error")
	if e.AppError != nil && e.AppError.Message != "" {
		return e.AppError.Message
	}
	return strings.ToLower(http.StatusText(e.StatusCode))
}
