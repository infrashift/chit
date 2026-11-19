package api

import (
	"net/http"

	"github.com/infrashift/chit/internal/assert"
)

func systemPing(w http.ResponseWriter, _ *http.Request) {
	assert.That(w != nil, "systemPing: nil writer")
	writeOK(w)
}

func systemClientConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		assert.That(w != nil, "systemClientConfig: nil writer")
		WriteJSON(w, http.StatusOK, map[string]any{
			"version": "0.1.0",
		})
	}
}
