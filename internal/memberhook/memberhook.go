// Package memberhook asks an external policy service whether a user may
// become a member of a channel. HALO's halod is that service (HALO PLAN.md 3.5
// item 9): a Profile post records the Tools and OKF namespaces it used, so a
// person added to the channel later would read output made with somebody
// else's access. Chit asks before it adds a member, and refuses the add when
// the answer is not a clear yes.
//
// The client authenticates with an OAuth2 client_credentials token from Ory
// Hydra. It keeps the token until shortly before it expires, and fetches a
// new one once when the hook answers 401.
//
// Every failure is a refusal: an error from Check means "do not add".
package memberhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Actor types in a Request. Chit calls a person "user"; the hook contract
// calls them "human".
const (
	ActorHuman = "human"
	ActorAgent = "agent"
	ActorBot   = "bot"
)

// Request is the body of POST <hook URL>.
type Request struct {
	ChannelID string `json:"channel_id"`
	// UserID is the Chit user ID of the member to add.
	UserID string `json:"user_id"`
	// KratosIdentityID is the member's Kratos identity, empty for a machine
	// actor.
	KratosIdentityID string `json:"kratos_identity_id"`
	// ActorType is the member's type: ActorHuman, ActorAgent or ActorBot.
	ActorType string `json:"actor_type"`
	// AddedBy is the Chit user ID of the actor who adds the member: for an
	// on-behalf-of request, the person.
	AddedBy string `json:"added_by"`
	// ViaActorID is the Chit user ID of the on-behalf-of client, empty when
	// the actor acts for itself.
	ViaActorID string `json:"via_actor_id"`
}

// Decision is the hook's answer.
type Decision struct {
	Allow bool
	// Reason is a short stable code, Detail a text for a person. Both are
	// empty when Allow is true.
	Reason string
	Detail string
}

// Bounds on what the hook may send back. A longer reason or detail is cut.
const (
	maxResponseBytes = 64 << 10
	MaxReasonBytes   = 64
	MaxDetailBytes   = 1024
)

// DefaultReason is the Reason of a refusal that names none.
const DefaultReason = "refused"

// tokenRefreshMargin is how long before its expiry a token is replaced.
const tokenRefreshMargin = 30 * time.Second

// Config configures a Hook.
type Config struct {
	// URL receives POST requests; it must be absolute http or https.
	URL string
	// Timeout bounds one Check, the token request and the retry included.
	Timeout time.Duration
	// TokenURL, ClientID, ClientSecret and Scopes are the Hydra
	// client_credentials settings of Chit's own OAuth2 client.
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	// HTTPClient sends the token and hook requests; nil means a new client.
	HTTPClient *http.Client
}

// Hook calls the membership hook. It is safe for concurrent use.
type Hook struct {
	url     string
	timeout time.Duration
	http    *http.Client
	creds   *clientcredentials.Config

	mu        sync.Mutex
	token     string
	refreshAt time.Time
}

// New returns a Hook, or an error when cfg is incomplete.
func New(cfg *Config) (*Hook, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("membership hook URL %q is not an absolute http or https URL", cfg.URL)
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("membership hook timeout must be positive")
	}
	if cfg.TokenURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("membership hook needs a token URL, client ID and client secret")
	}
	client := cfg.HTTPClient
	if client == nil {
		// No redirects: a redirect would carry the bearer token to another
		// URL, and a 3xx is not an answer.
		client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	}
	return &Hook{
		url:     cfg.URL,
		timeout: cfg.Timeout,
		http:    client,
		creds: &clientcredentials.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			TokenURL:     cfg.TokenURL,
			Scopes:       cfg.Scopes,
			// Hydra accepts only the client's registered
			// token_endpoint_auth_method; auto-detect finds it.
			AuthStyle: oauth2.AuthStyleAutoDetect,
		},
	}, nil
}

// Check asks the hook about req. It returns an error, and no decision, when
// the hook cannot be asked or does not answer as the contract says: a status
// other than 200, a timeout, a network error or a body that is not the
// expected JSON. The caller must refuse the add then.
func (h *Hook) Check(ctx context.Context, req *Request) (Decision, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	body, err := json.Marshal(req)
	if err != nil {
		return Decision{}, fmt.Errorf("encode membership hook request: %w", err)
	}

	token, err := h.accessToken(ctx)
	if err != nil {
		return Decision{}, err
	}
	status, d, err := h.ask(ctx, token, body)
	if err == nil && status == http.StatusUnauthorized {
		// The token may have been revoked or the hook's key rotated: one
		// new token, one more try.
		h.forget(token)
		if token, err = h.accessToken(ctx); err != nil {
			return Decision{}, err
		}
		status, d, err = h.ask(ctx, token, body)
	}
	if err != nil {
		return Decision{}, err
	}
	if status != http.StatusOK {
		return Decision{}, fmt.Errorf("membership hook answered %d", status)
	}
	return d, nil
}

// ask sends one request to the hook. It returns the status, and the decoded
// decision when the status is 200.
func (h *Hook) ask(ctx context.Context, token string, body []byte) (int, Decision, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return 0, Decision{}, fmt.Errorf("create membership hook request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	resp, err := h.http.Do(httpReq)
	if err != nil {
		return 0, Decision{}, fmt.Errorf("call membership hook: %w", err)
	}
	// The rest of the body is read so that the connection can be reused.
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, Decision{}, nil
	}
	d, err := decode(resp.Body)
	return resp.StatusCode, d, err
}

// decode reads the answer: {"allow":true}, or {"allow":false,"reason":...,
// "detail":...}. allow must be present and a boolean.
func decode(r io.Reader) (Decision, error) {
	var answer struct {
		Allow  *bool  `json:"allow"`
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	data, err := io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
	if err != nil {
		return Decision{}, fmt.Errorf("read membership hook answer: %w", err)
	}
	if len(data) > maxResponseBytes {
		return Decision{}, errors.New("membership hook answer is too large")
	}
	if err := json.Unmarshal(data, &answer); err != nil {
		return Decision{}, fmt.Errorf("membership hook answer is not valid JSON: %w", err)
	}
	if answer.Allow == nil {
		return Decision{}, errors.New("membership hook answer has no allow")
	}
	if *answer.Allow {
		return Decision{Allow: true}, nil
	}
	d := Decision{Reason: cut(answer.Reason, MaxReasonBytes), Detail: cut(answer.Detail, MaxDetailBytes)}
	if d.Reason == "" {
		d.Reason = DefaultReason
	}
	return d, nil
}

// cut shortens s to at most n bytes, on a rune boundary, and drops invalid
// UTF-8.
func cut(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// accessToken returns the cached token, or fetches one when there is none
// or it is near its expiry.
func (h *Hook) accessToken(ctx context.Context) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if h.token != "" && now.Before(h.refreshAt) {
		return h.token, nil
	}
	tok, err := h.creds.Token(context.WithValue(ctx, oauth2.HTTPClient, h.http))
	if err != nil {
		return "", fmt.Errorf("obtain membership hook token: %w", err)
	}
	if tok.AccessToken == "" {
		return "", errors.New("obtain membership hook token: empty access token")
	}
	h.token = tok.AccessToken
	h.refreshAt = refreshTime(now, tok.Expiry)
	return h.token, nil
}

// refreshTime is when a token that expires at expiry is replaced:
// tokenRefreshMargin before it, or halfway through a shorter life. A token
// with no expiry is kept until the hook refuses it.
func refreshTime(now, expiry time.Time) time.Time {
	if expiry.IsZero() {
		return now.Add(100 * 365 * 24 * time.Hour)
	}
	margin := min(tokenRefreshMargin, expiry.Sub(now)/2)
	return expiry.Add(-margin)
}

// forget drops token from the cache, unless another request has already
// replaced it.
func (h *Hook) forget(token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.token == token {
		h.token = ""
	}
}
