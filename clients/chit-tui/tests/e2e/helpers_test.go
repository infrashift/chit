//go:build e2e

package e2e_test

import (
	"os"

	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// Constants matching chit server's seed-uat script.
const (
	defaultServerURL = "http://localhost:8065"
	headerName       = "X-User-Id"

	aliceKratosID = "a11ce000-0000-4000-a000-000000000001"
	bobKratosID   = "b0b00000-0000-4000-a000-000000000002"
	chadKratosID  = "c4ad0000-0000-4000-a000-000000000003"
	dianaKratosID = "d1a4a000-0000-4000-a000-000000000004"
	// agentKratosID is chit-agent, a user with actor_type "agent".
	agentKratosID = "a9e47000-0000-4000-a000-000000000006"
	// halodClientID is the machine actor that the TUI Makefile allows to
	// post on behalf of a person (CHIT_ON_BEHALF_OF_CLIENTS).
	halodClientID = "tui-e2e-halod"
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
