package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// APIError wraps a server-side AppError with request context.
type APIError struct {
	StatusCode int
	AppError   *model.AppError
}

func (e *APIError) Error() string {
	assert.That(e != nil, "APIError.Error: nil error")
	if e.AppError != nil {
		return fmt.Sprintf("api: %d %s", e.StatusCode, e.AppError.Message)
	}
	return fmt.Sprintf("api: %d", e.StatusCode)
}

// IsUnauthorized checks if an error is a 401 API error.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		assert.That(apiErr != nil, "IsUnauthorized: errors.As matched a nil *APIError")
		return apiErr.StatusCode == 401
	}
	return false
}

// parseErrorResponse reads the response body and returns an APIError.
func parseErrorResponse(resp *http.Response) *APIError {
	assert.That(resp != nil && resp.Body != nil, "parseErrorResponse: no response body")
	assert.That(resp.StatusCode >= 400, "parseErrorResponse: not an error status")
	defer errlog.Close(resp.Body, "api error response body")

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			AppError:   &model.AppError{Message: "failed to read error response"},
		}
	}

	var appErr model.AppError
	if err := json.Unmarshal(body, &appErr); err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			AppError:   &model.AppError{Message: string(body)},
		}
	}

	appErr.StatusCode = resp.StatusCode
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		AppError:   &appErr,
	}
	assert.That(apiErr.AppError.StatusCode == apiErr.StatusCode, "parseErrorResponse: status codes disagree")
	return apiErr
}
