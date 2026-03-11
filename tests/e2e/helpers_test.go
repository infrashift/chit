//go:build e2e

package e2e_test

import (
	"os"

	"github.com/infrashift/chit-tui/internal/api"
	"github.com/infrashift/chit-tui/internal/ws"
)

// Constants matching chit server's seed-uat script.
const (
	defaultServerURL = "http://localhost:8065"
	headerName       = "X-User-Id"

	aliceKratosID = "a11ce000-0000-4000-a000-000000000001"
	bobKratosID   = "b0b00000-0000-4000-a000-000000000002"
	chadKratosID  = "c4ad0000-0000-4000-a000-000000000003"
)

func serverURL() string {
	if v := os.Getenv("CHIT_E2E_SERVER_URL"); v != "" {
		return v
	}
	return defaultServerURL
}

func newTestClient(kratosID string) api.ChitClient {
	return api.NewClientWithHeader(serverURL(), kratosID, headerName)
}

func newTestWSClient(kratosID string, bufSize int) ws.WSClient {
	base := serverURL()
	wsURL := "ws" + base[len("http"):] + "/api/v1/websocket"
	return ws.NewWSClientWithHeader(wsURL, kratosID, bufSize, headerName)
}
