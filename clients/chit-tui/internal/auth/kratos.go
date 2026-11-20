package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
)

// LoginFlow represents a Kratos self-service login flow.
type LoginFlow struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Session represents a Kratos session returned after login.
type Session struct {
	Token     string    `json:"session_token"`
	ExpiresAt time.Time `json:"expires_at"`
	Identity  Identity  `json:"identity"`
}

// kratosLoginResponse maps the Kratos API response for login submission.
type kratosLoginResponse struct {
	SessionToken string `json:"session_token"`
	Session      struct {
		ExpiresAt time.Time `json:"expires_at"`
		Identity  Identity  `json:"identity"`
	} `json:"session"`
}

// Identity represents a Kratos identity.
type Identity struct {
	ID     string `json:"id"`
	Traits Traits `json:"traits"`
}

// Traits holds identity traits from Kratos.
type Traits struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// KratosError represents an error from the Kratos API.
type KratosError struct {
	StatusCode int
	Message    string
}

func (e *KratosError) Error() string {
	assert.That(e != nil, "KratosError.Error: nil error")
	return fmt.Sprintf("kratos: %d %s", e.StatusCode, e.Message)
}

// IsSessionRejected reports whether Kratos itself refused a session. Any
// other failure, such as Kratos being unreachable or erroring, says nothing
// about whether the session is still good.
func IsSessionRejected(err error) bool {
	var ke *KratosError
	if !errors.As(err, &ke) {
		return false
	}
	assert.That(ke != nil, "IsSessionRejected: errors.As matched a nil *KratosError")
	return ke.StatusCode == http.StatusUnauthorized || ke.StatusCode == http.StatusForbidden
}

// KratosClient is an HTTP client for Kratos self-service flows.
type KratosClient struct {
	baseURL string
	http    *http.Client
}

// NewKratosClient creates a new KratosClient. baseURL should be "{ServerURL}/kratos".
func NewKratosClient(baseURL string) *KratosClient {
	assert.That(baseURL != "", "NewKratosClient: empty base URL")
	c := &KratosClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	assert.That(c.http.Timeout > 0, "NewKratosClient: requests must time out")
	return c
}

// InitLoginFlow initiates a Kratos self-service login flow for API clients.
func (c *KratosClient) InitLoginFlow(ctx context.Context) (*LoginFlow, error) {
	assert.That(ctx != nil, "InitLoginFlow: nil context")
	assert.That(c.http != nil && c.baseURL != "", "InitLoginFlow: client not built by NewKratosClient")
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/self-service/login/api", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	assert.That(resp != nil && resp.Body != nil, "kratos: no response without an error")
	defer errlog.Close(resp.Body, "kratos response body")

	if resp.StatusCode >= 400 {
		return nil, c.parseError(resp)
	}

	var flow LoginFlow
	if err := json.NewDecoder(resp.Body).Decode(&flow); err != nil {
		return nil, fmt.Errorf("kratos: failed to decode login flow: %w", err)
	}
	return &flow, nil
}

// SubmitLogin submits credentials to an active login flow.
func (c *KratosClient) SubmitLogin(ctx context.Context, flowID, identifier, password string) (*Session, error) {
	assert.That(ctx != nil, "SubmitLogin: nil context")
	assert.That(c.http != nil && c.baseURL != "", "SubmitLogin: client not built by NewKratosClient")
	body := map[string]string{
		"method":     "password",
		"identifier": identifier,
		"password":   password,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	assert.That(len(data) > 0, "SubmitLogin: empty request body")

	url := fmt.Sprintf("%s/self-service/login?flow=%s", c.baseURL, flowID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	assert.That(resp != nil && resp.Body != nil, "kratos: no response without an error")
	defer errlog.Close(resp.Body, "kratos response body")

	if resp.StatusCode >= 400 {
		return nil, c.parseError(resp)
	}

	var loginResp kratosLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, fmt.Errorf("kratos: failed to decode login response: %w", err)
	}

	return &Session{
		Token:     loginResp.SessionToken,
		ExpiresAt: loginResp.Session.ExpiresAt,
		Identity:  loginResp.Session.Identity,
	}, nil
}

// CheckSession validates a session token against Kratos.
func (c *KratosClient) CheckSession(ctx context.Context, token string) (*Identity, error) {
	assert.That(ctx != nil, "CheckSession: nil context")
	assert.That(c.http != nil && c.baseURL != "", "CheckSession: client not built by NewKratosClient")
	assert.That(token != "", "CheckSession: empty token")
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/sessions/whoami", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Session-Token", token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	assert.That(resp != nil && resp.Body != nil, "kratos: no response without an error")
	defer errlog.Close(resp.Body, "kratos response body")

	if resp.StatusCode >= 400 {
		return nil, c.parseError(resp)
	}

	var session struct {
		Identity Identity `json:"identity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("kratos: failed to decode session: %w", err)
	}
	return &session.Identity, nil
}

func (c *KratosClient) parseError(resp *http.Response) error {
	assert.That(resp != nil && resp.Body != nil, "parseError: no response body")
	assert.That(resp.StatusCode >= 400, "parseError: not an error status")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &KratosError{StatusCode: resp.StatusCode, Message: "failed to read error response"}
	}

	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
		UI      struct {
			Messages []struct {
				Text string `json:"text"`
			} `json:"messages"`
			// A rejected form reports on its fields, such as a password
			// that is too short, rather than in a top-level message.
			Nodes []struct {
				Messages []struct {
					Text string `json:"text"`
				} `json:"messages"`
			} `json:"nodes"`
		} `json:"ui"`
	}
	if err := json.Unmarshal(body, &errResp); err != nil {
		return &KratosError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	msg := errResp.Error.Message
	if msg == "" {
		msg = errResp.Message
	}
	if msg == "" {
		for _, m := range errResp.UI.Messages {
			if m.Text != "" {
				msg = m.Text
				break
			}
		}
	}
	for _, n := range errResp.UI.Nodes {
		for _, m := range n.Messages {
			if msg == "" && m.Text != "" {
				msg = m.Text
			}
		}
	}
	if msg == "" {
		msg = string(body)
	}
	assert.That(msg != "" || len(body) == 0, "parseError: a non-empty body produced no message")
	return &KratosError{StatusCode: resp.StatusCode, Message: msg}
}
