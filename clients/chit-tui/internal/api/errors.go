package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// APIError wraps a server-side AppError with request context.
type APIError struct {
	StatusCode int
	AppError   *model.AppError
}

func (e *APIError) Error() string {
	if e.AppError != nil {
		return fmt.Sprintf("api: %d %s", e.StatusCode, e.AppError.Message)
	}
	return fmt.Sprintf("api: %d", e.StatusCode)
}

// IsUnauthorized checks if an error is a 401 API error.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 401
	}
	return false
}

// parseErrorResponse reads the response body and returns an APIError.
func parseErrorResponse(resp *http.Response) *APIError {
	defer func() { _ = resp.Body.Close() }()

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
	return &APIError{
		StatusCode: resp.StatusCode,
		AppError:   &appErr,
	}
}
