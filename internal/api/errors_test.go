package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/infrashift/chit-tui/internal/model"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "with app error",
			err:  &APIError{StatusCode: 400, AppError: &model.AppError{Message: "bad request"}},
			want: "api: 400 bad request",
		},
		{
			name: "without app error",
			err:  &APIError{StatusCode: 500},
			want: "api: 500",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseErrorResponse_ValidJSON(t *testing.T) {
	body := `{"id":"test.error","message":"not found","status_code":404}`
	resp := &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	apiErr := parseErrorResponse(resp)
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.AppError.Message != "not found" {
		t.Errorf("Message = %q, want %q", apiErr.AppError.Message, "not found")
	}
	if apiErr.AppError.ID != "test.error" {
		t.Errorf("ID = %q, want %q", apiErr.AppError.ID, "test.error")
	}
}

func TestParseErrorResponse_InvalidJSON(t *testing.T) {
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("internal server error")),
	}

	apiErr := parseErrorResponse(resp)
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
	if apiErr.AppError.Message != "internal server error" {
		t.Errorf("Message = %q", apiErr.AppError.Message)
	}
}

func TestIsUnauthorized(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "401 APIError",
			err:  &APIError{StatusCode: 401, AppError: &model.AppError{Message: "unauthorized"}},
			want: true,
		},
		{
			name: "403 APIError",
			err:  &APIError{StatusCode: 403, AppError: &model.AppError{Message: "forbidden"}},
			want: false,
		},
		{
			name: "wrapped 401",
			err:  fmt.Errorf("wrapped: %w", &APIError{StatusCode: 401}),
			want: true,
		},
		{
			name: "non-APIError",
			err:  fmt.Errorf("some network error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnauthorized(tt.err); got != tt.want {
				t.Errorf("IsUnauthorized() = %v, want %v", got, tt.want)
			}
		})
	}
}
