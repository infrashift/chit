package api

import (
	"net/http"

	"github.com/infrashift/chit/internal/app"
)

func systemPing(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
}

func systemClientConfig(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]any{
			"version": "0.1.0",
		})
	}
}
