package api

import (
	"net/http"
)

func systemPing(w http.ResponseWriter, _ *http.Request) {
	writeOK(w)
}

func systemClientConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]any{
			"version": "0.1.0",
		})
	}
}
