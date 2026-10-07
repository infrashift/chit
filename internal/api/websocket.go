package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
	ws "github.com/infrashift/chit/internal/websocket"
)

// originAllowed permits requests with no Origin header (non-browser clients)
// and browser requests whose Origin is on the configured allowlist.
func originAllowed(r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, o := range allowed {
		if o == "*" || strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

func handleWebSocket(a *app.App) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return originAllowed(r, a.Config.AllowedOrigins)
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if user == nil {
			WriteError(w, model.NewUnauthorizedError("handleWebSocket", "not authenticated"))
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// The upgrader has already answered the client. This is almost
			// always a rejected origin or a non-WebSocket request: the
			// client's problem, not the server's.
			slog.Warn("websocket: upgrade failed", "user_id", user.ID, "error", err)
			return
		}

		client := ws.NewClient(a.Hub, conn, user.ID, a.Config.WSPingInterval, a.Config.WSWriteTimeout)
		a.Hub.Register(client)
		client.Start()

		slog.Info("websocket: client connected", "user_id", user.ID)
	}
}
