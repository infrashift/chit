package api

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/infrashift/chit/internal/app"
	ws "github.com/infrashift/chit/internal/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in dev; restrict in production
	},
}

func handleWebSocket(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("websocket: upgrade failed", "error", err)
			return
		}

		client := ws.NewClient(a.Hub, conn, user.ID, a.Config.WSPingInterval, a.Config.WSWriteTimeout)
		a.Hub.Register(client)

		slog.Info("websocket: client connected", "user_id", user.ID)
	}
}
