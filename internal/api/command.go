package api

import (
	"net/http"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
)

func listCommands(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "listCommands: nil app")
	return func(w http.ResponseWriter, r *http.Request) {
		if a.CommandRegistry == nil {
			WriteJSON(w, http.StatusOK, []any{})
			return
		}
		WriteJSON(w, http.StatusOK, a.CommandRegistry.All())
	}
}
