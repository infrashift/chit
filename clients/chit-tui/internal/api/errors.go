package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// Bounds on what an error response can make the client hold or wait.
const (
	// maxErrorBody is the most of an error body that the client reads.
	maxErrorBody = 64 << 10
	// MaxRetryAfter caps the wait that a Retry-After header can ask for.
	MaxRetryAfter = 60 * time.Second
)

// APIError wraps a server-side AppError with request context.
type APIError struct {
	StatusCode int
	AppError   *model.AppError
	// RetryAfter is the wait that the server asked for with Retry-After,
	// at most MaxRetryAfter. It is 0 when the response had no such header.
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	assert.That(e != nil, "APIError.Error: nil error")
	if e.AppError == nil {
		return fmt.Sprintf("api: %d", e.StatusCode)
	}
	if e.AppError.Detail != "" {
		return fmt.Sprintf("api: %d %s: %s", e.StatusCode, e.AppError.Message, e.AppError.Detail)
	}
	return fmt.Sprintf("api: %d %s", e.StatusCode, e.AppError.Message)
}

// ID returns the server's error ID, or "" when the body had none.
func (e *APIError) ID() string {
	assert.That(e != nil, "APIError.ID: nil error")
	if e.AppError == nil {
		return ""
	}
	return e.AppError.ID
}

// IsUnauthorized checks if an error is a 401 API error.
func IsUnauthorized(err error) bool {
	return StatusOf(err) == http.StatusUnauthorized
}

// StatusOf returns the HTTP status of an API error, or 0 for any other
// error.
func StatusOf(err error) int {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return 0
	}
	assert.That(apiErr != nil, "StatusOf: errors.As matched a nil *APIError")
	return apiErr.StatusCode
}

// RetryAfterOf returns the wait that a 429 asked for, and false for any
// other error.
func RetryAfterOf(err error) (time.Duration, bool) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		return 0, false
	}
	assert.That(apiErr.RetryAfter >= 0 && apiErr.RetryAfter <= MaxRetryAfter, "RetryAfterOf: wait outside its bounds")
	return apiErr.RetryAfter, true
}

// parseErrorResponse reads the response body and returns an APIError.
func parseErrorResponse(resp *http.Response) *APIError {
	assert.That(resp != nil && resp.Body != nil, "parseErrorResponse: no response body")
	assert.That(resp.StatusCode >= 400, "parseErrorResponse: not an error status")
	defer errlog.Close(resp.Body, "api error response body")

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil {
		apiErr.AppError = &model.AppError{Message: "failed to read error response"}
		return apiErr
	}

	var appErr model.AppError
	if err := json.Unmarshal(body, &appErr); err != nil {
		apiErr.AppError = &model.AppError{Message: strings.TrimSpace(string(body))}
		return apiErr
	}
	appErr.StatusCode = resp.StatusCode
	apiErr.AppError = &appErr
	assert.That(apiErr.AppError.StatusCode == apiErr.StatusCode, "parseErrorResponse: status codes disagree")
	return apiErr
}

// parseRetryAfter reads a Retry-After value: whole seconds or an HTTP date.
// It returns 0 for an empty or bad value, and at most MaxRetryAfter.
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.Atoi(v); err == nil {
		d = time.Duration(secs) * time.Second
	} else if at, err := http.ParseTime(v); err == nil {
		d = at.Sub(now)
	}
	d = min(max(d, 0), MaxRetryAfter)
	assert.That(d >= 0 && d <= MaxRetryAfter, "parseRetryAfter: wait outside its bounds")
	return d
}
