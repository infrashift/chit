package login

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
)

// LoginSuccessMsg is sent when login succeeds.
type LoginSuccessMsg struct {
	Token     string
	ExpiresAt string
}

// LoginErrorMsg is sent when login fails.
type LoginErrorMsg struct {
	Err error
}

// InitLoginAndSubmit initiates a Kratos login flow and submits credentials.
func InitLoginAndSubmit(kratosClient *auth.KratosClient, identifier, password string) tea.Cmd {
	assert.That(identifier != "" && password != "", "login.InitLoginAndSubmit: empty credentials")
	return func() tea.Msg {
		assert.That(kratosClient != nil, "login.InitLoginAndSubmit: nil Kratos client")
		ctx := context.Background()

		flow, err := kratosClient.InitLoginFlow(ctx)
		if err != nil {
			return LoginErrorMsg{Err: err}
		}

		assert.That(flow != nil, "login.InitLoginAndSubmit: nil flow without an error")
		sess, err := kratosClient.SubmitLogin(ctx, flow.ID, identifier, password)
		if err != nil {
			return LoginErrorMsg{Err: err}
		}
		assert.That(sess != nil, "login.InitLoginAndSubmit: nil session without an error")

		return LoginSuccessMsg{
			Token:     sess.Token,
			ExpiresAt: sess.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
}
