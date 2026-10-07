package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/infrashift/chit/internal/model"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		writeBody(w, data)
	}
}

// WriteError writes a structured error response.
func WriteError(w http.ResponseWriter, appErr *model.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.StatusCode)
	writeBody(w, appErr)
}

// writeBody encodes data after the header is sent. The status can no longer
// change at that point, so a failure (almost always the client going away)
// is logged rather than returned.
func writeBody(w http.ResponseWriter, data any) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Debug("write response body", "error", err)
	}
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
