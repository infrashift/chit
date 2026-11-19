package model

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/errlog"
)

// AppError represents a structured application error returned to clients.
type AppError struct {
	ID            string `json:"id"`
	Message       string `json:"message"`
	DetailedError string `json:"detailed_error,omitempty"`
	StatusCode    int    `json:"status_code"`
	Where         string `json:"-"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Where, e.Message)
}

func (e *AppError) ToJSON() string {
	assert.That(e != nil, "AppError.ToJSON: nil error")
	// An AppError holds only strings and an int, so Marshal cannot fail;
	// a failure is logged and the result is then "".
	b, err := json.Marshal(e)
	errlog.Warn(err, "marshal app error")
	return string(b)
}

func NewAppError(where, message, detailedError string, statusCode int) *AppError {
	assert.That(where != "", "NewAppError: empty where")
	assert.That(statusCode >= 400 && statusCode <= 599, "NewAppError: status code is not an error")
	return &AppError{
		ID:            where,
		Where:         where,
		Message:       message,
		DetailedError: detailedError,
		StatusCode:    statusCode,
	}
}

func NewNotFoundError(where, id string) *AppError {
	assert.That(where != "", "NewNotFoundError: empty where")
	return NewAppError(where, "resource not found", fmt.Sprintf("id=%s", id), http.StatusNotFound)
}

func NewBadRequestError(where, message string) *AppError {
	assert.That(where != "" && message != "", "NewBadRequestError: empty where or message")
	return NewAppError(where, message, "", http.StatusBadRequest)
}

func NewUnauthorizedError(where, message string) *AppError {
	assert.That(where != "" && message != "", "NewUnauthorizedError: empty where or message")
	return NewAppError(where, message, "", http.StatusUnauthorized)
}

func NewForbiddenError(where, message string) *AppError {
	assert.That(where != "" && message != "", "NewForbiddenError: empty where or message")
	return NewAppError(where, message, "", http.StatusForbidden)
}

func NewConflictError(where, message string) *AppError {
	assert.That(where != "" && message != "", "NewConflictError: empty where or message")
	return NewAppError(where, message, "", http.StatusConflict)
}

func NewInternalError(where string, err error) *AppError {
	assert.That(where != "", "NewInternalError: empty where")
	detailedErr := ""
	if err != nil {
		detailedErr = err.Error()
	}
	return NewAppError(where, "internal server error", detailedErr, http.StatusInternalServerError)
}
