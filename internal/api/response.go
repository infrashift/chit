package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/infrashift/chit/internal/model"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// WriteError writes a structured error response.
func WriteError(w http.ResponseWriter, appErr *model.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.StatusCode)
	json.NewEncoder(w).Encode(appErr)
}

// WriteAppError writes err preserving its AppError status code (403, 404, …)
// and falls back to a 500 for unknown errors.
func WriteAppError(w http.ResponseWriter, where string, err error) {
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		WriteError(w, appErr)
		return
	}
	WriteError(w, model.NewInternalError(where, err))
}
