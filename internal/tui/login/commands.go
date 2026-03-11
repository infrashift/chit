package login

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit-tui/internal/auth"
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
	return func() tea.Msg {
		ctx := context.Background()

		flow, err := kratosClient.InitLoginFlow(ctx)
		if err != nil {
			return LoginErrorMsg{Err: err}
		}

		sess, err := kratosClient.SubmitLogin(ctx, flow.ID, identifier, password)
		if err != nil {
			return LoginErrorMsg{Err: err}
		}

		return LoginSuccessMsg{
			Token:     sess.Token,
			ExpiresAt: sess.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
}

// ValidateSession checks if a stored session token is still valid.
func ValidateSession(kratosClient *auth.KratosClient, token string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		_, err := kratosClient.CheckSession(ctx, token)
		if err != nil {
			return LoginErrorMsg{Err: err}
		}
		return LoginSuccessMsg{Token: token}
	}
}
