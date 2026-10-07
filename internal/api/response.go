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

// writeOK answers 200 with {"status":"OK"}, the body of every endpoint that
// has nothing else to return.
func writeOK(w http.ResponseWriter) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
}

// writeList answers 200 with items, as [] rather than null when there are
// none: a nil slice encodes as null, which clients then have to special-case.
func writeList[T any](w http.ResponseWriter, items []T) {
	if items == nil {
		items = []T{}
	}
	WriteJSON(w, http.StatusOK, items)
}

// WriteError writes a structured error response.
//
// A 5xx is logged with its detail and sent without it. DetailedError on a
// server error is the raw cause (pgx and SQL text, upstream bodies), which
// tells a client nothing it can act on and an attacker something about the
// schema.
func WriteError(w http.ResponseWriter, appErr *model.AppError) {
	if appErr.StatusCode >= http.StatusInternalServerError && appErr.DetailedError != "" {
		slog.Error("request failed", "where", appErr.Where, "error", appErr.DetailedError)
		clean := *appErr
		clean.DetailedError = ""
		appErr = &clean
	}
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

// maxBodyBytes caps every JSON request body. Nothing the API accepts comes
// near it: a post is at most 64 KiB of content.
const maxBodyBytes = 1 << 20

// decodeBody reads r's JSON body into v. On failure it writes the error (413
// for an oversized body, 400 otherwise) and returns false.
func decodeBody(w http.ResponseWriter, r *http.Request, v any, where string) bool {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(v)
	if err == nil {
		return true
	}
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		WriteError(w, model.NewAppError(where, "request body too large", "", http.StatusRequestEntityTooLarge))
		return false
	}
	WriteError(w, model.NewBadRequestError(where, "invalid request body"))
	return false
}
