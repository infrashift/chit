package model

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	b, _ := json.Marshal(e)
	return string(b)
}

func NewAppError(where, message, detailedError string, statusCode int) *AppError {
	return &AppError{
		ID:            where,
		Where:         where,
		Message:       message,
		DetailedError: detailedError,
		StatusCode:    statusCode,
	}
}

func NewNotFoundError(where, id string) *AppError {
	return NewAppError(where, "resource not found", fmt.Sprintf("id=%s", id), http.StatusNotFound)
}

func NewBadRequestError(where, message string) *AppError {
	return NewAppError(where, message, "", http.StatusBadRequest)
}

func NewUnauthorizedError(where, message string) *AppError {
	return NewAppError(where, message, "", http.StatusUnauthorized)
}

func NewForbiddenError(where, message string) *AppError {
	return NewAppError(where, message, "", http.StatusForbidden)
}

func NewConflictError(where, message string) *AppError {
	return NewAppError(where, message, "", http.StatusConflict)
}

func NewInternalError(where string, err error) *AppError {
	detailedErr := ""
	if err != nil {
		detailedErr = err.Error()
	}
	return NewAppError(where, "internal server error", detailedErr, http.StatusInternalServerError)
}
