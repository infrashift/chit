package api

import (
	"net/http"

	"github.com/infrashift/chit/internal/app"
)

func listCommands(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.CommandRegistry == nil {
			WriteJSON(w, http.StatusOK, []any{})
			return
		}
		WriteJSON(w, http.StatusOK, a.CommandRegistry.All())
	}
}
