// Package chitclient is an HTTP/WebSocket client for chitd's REST API,
// shared by the headless binaries (chit-claude, chit-mcp) that act as agent
// users. It supports two authentication modes:
//
//   - OAuth2 client credentials (NewOAuth): the agent obtains a short-lived
//     access token from Ory Hydra and presents it to Oathkeeper, which
//     introspects it, enforces the audience and scope, and forwards the
//     client ID to chitd. This is the supported mode for agents.
//   - Trusted proxy header (New): the agent asserts a Kratos ID directly to
//     chitd, bypassing Oathkeeper. Retained for local development and tests;
//     it carries no per-agent credential, only a shared secret.
package chitclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/infrashift/chit/internal/model"
)

// OAuthConfig describes the client-credentials grant an agent uses to obtain
// access tokens from Hydra.
type OAuthConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes must cover what the agent does: chit:read to listen and fetch,
	// chit:write to post. Oathkeeper rejects a token missing the scope its
	// rule requires.
	Scopes []string
	// Audience names the application the token is for. Oathkeeper's rules
	// require it, so a token minted for another app cannot be replayed here.
	Audience string
}

// Client talks to chitd over its HTTP API and WebSocket.
type Client struct {
	baseURL     string
	kratosID    string
	proxySecret string
	tokens      oauth2.TokenSource
	http        *http.Client
}

// New creates a client that authenticates with the trusted proxy header.
func New(baseURL, kratosID, proxySecret string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		kratosID:    kratosID,
		proxySecret: proxySecret,
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}

// NewOAuth creates a client that authenticates with Hydra-issued access
// tokens. baseURL should point at Oathkeeper, not chitd: the token is only
// meaningful to the proxy that introspects it.
//
// The returned token source caches the token and fetches a new one when it
// expires, so callers never deal with refresh.
func NewOAuth(baseURL string, cfg OAuthConfig) *Client {
	ccfg := &clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
		// Hydra enforces the client's registered token_endpoint_auth_method
		// and rejects the other outright, so do not pin one here: auto-detect
		// probes and caches whichever the client was registered with
		// (client_secret_basic or client_secret_post).
		AuthStyle: oauth2.AuthStyleAutoDetect,
	}
	if cfg.Audience != "" {
		ccfg.EndpointParams = url.Values{"audience": {cfg.Audience}}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		tokens:  ccfg.TokenSource(context.Background()),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// authHeaders returns the credentials for one request. In OAuth2 mode this
// may fetch a fresh token, so it can fail — a request must not be sent
// unauthenticated when it does.
func (c *Client) authHeaders() (http.Header, error) {
	h := http.Header{}
	if c.tokens != nil {
		token, err := c.tokens.Token()
		if err != nil {
			return nil, fmt.Errorf("obtain access token: %w", err)
		}
		h.Set("Authorization", token.Type()+" "+token.AccessToken)
		return h, nil
	}
	h.Set("X-User-Id", c.kratosID)
	if c.proxySecret != "" {
		h.Set("X-Proxy-Secret", c.proxySecret)
	}
	return h, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	headers, err := c.authHeaders()
	if err != nil {
		return err
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode %s %s response: %w", method, path, err)
		}
	}
	return nil
}

// pageQuery renders page/per_page query params understood by chitd's list
// endpoints.
func pageQuery(page, perPage int) string {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))
	return "?" + q.Encode()
}

// ─── Users ───────────────────────────────────────────────────────

// Me returns the agent's own user record (provisioning it on first contact).
func (c *Client) Me(ctx context.Context) (*model.User, error) {
	var user model.User
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUser returns a user's (sanitized) profile by internal ID.
func (c *Client) GetUser(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/"+id, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername looks a user up by username.
func (c *Client) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/username/"+username, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ─── Teams ───────────────────────────────────────────────────────

// GetAllTeams lists teams.
func (c *Client) GetAllTeams(ctx context.Context, page, perPage int) ([]*model.Team, error) {
	var teams []*model.Team
	if err := c.do(ctx, http.MethodGet, "/api/v1/teams"+pageQuery(page, perPage), nil, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// GetTeam returns one team by ID.
func (c *Client) GetTeam(ctx context.Context, id string) (*model.Team, error) {
	var team model.Team
	if err := c.do(ctx, http.MethodGet, "/api/v1/teams/"+id, nil, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// ─── Channels ────────────────────────────────────────────────────

// GetChannelsForTeam lists a team's channels visible to the agent.
func (c *Client) GetChannelsForTeam(ctx context.Context, teamID string, page, perPage int) ([]*model.Channel, error) {
	var channels []*model.Channel
	if err := c.do(ctx, http.MethodGet, "/api/v1/teams/"+teamID+"/channels"+pageQuery(page, perPage), nil, &channels); err != nil {
		return nil, err
	}
	return channels, nil
}

// GetChannel returns one channel by ID.
func (c *Client) GetChannel(ctx context.Context, id string) (*model.Channel, error) {
	var channel model.Channel
	if err := c.do(ctx, http.MethodGet, "/api/v1/channels/"+id, nil, &channel); err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetChannelMembers lists a channel's members.
func (c *Client) GetChannelMembers(ctx context.Context, channelID string, page, perPage int) ([]*model.ChannelMember, error) {
	var members []*model.ChannelMember
	if err := c.do(ctx, http.MethodGet, "/api/v1/channels/"+channelID+"/members"+pageQuery(page, perPage), nil, &members); err != nil {
		return nil, err
	}
	return members, nil
}

// ViewChannel marks a channel as viewed for the agent, clearing unreads.
func (c *Client) ViewChannel(ctx context.Context, channelID string) error {
	return c.do(ctx, http.MethodPost, "/api/v1/channels/"+channelID+"/members/me/view", nil, nil)
}

// ─── Posts ───────────────────────────────────────────────────────

// GetChannelPosts returns a channel's posts. since (unix ms) is optional; 0
// means no lower bound.
func (c *Client) GetChannelPosts(ctx context.Context, channelID string, page, perPage int, since int64) (*model.PostList, error) {
	path := "/api/v1/channels/" + channelID + "/posts" + pageQuery(page, perPage)
	if since > 0 {
		path += "&since=" + strconv.FormatInt(since, 10)
	}
	var list model.PostList
	if err := c.do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// GetPost returns one post by ID.
func (c *Client) GetPost(ctx context.Context, id string) (*model.Post, error) {
	var post model.Post
	if err := c.do(ctx, http.MethodGet, "/api/v1/posts/"+id, nil, &post); err != nil {
		return nil, err
	}
	return &post, nil
}

// GetPinnedPosts returns a channel's pinned posts.
func (c *Client) GetPinnedPosts(ctx context.Context, channelID string) (*model.PostList, error) {
	var list model.PostList
	if err := c.do(ctx, http.MethodGet, "/api/v1/channels/"+channelID+"/pinned", nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// CreatePost posts a message as the agent. rootID may be empty for a root post.
func (c *Client) CreatePost(ctx context.Context, channelID, rootID, content string, props map[string]any) (*model.Post, error) {
	post := &model.Post{
		ChannelID: channelID,
		RootID:    rootID,
		Content:   content,
		Props:     props,
	}
	var saved model.Post
	if err := c.do(ctx, http.MethodPost, "/api/v1/posts", post, &saved); err != nil {
		return nil, err
	}
	return &saved, nil
}

// SearchPosts searches posts across all channels the agent is a member of.
// Provide terms, tagIDs, or both.
func (c *Client) SearchPosts(ctx context.Context, terms string, tagIDs []string, page, perPage int) (*model.PostList, error) {
	body := map[string]any{
		"terms":    terms,
		"tag_ids":  tagIDs,
		"page":     page,
		"per_page": perPage,
	}
	var list model.PostList
	if err := c.do(ctx, http.MethodPost, "/api/v1/posts/search", body, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// ─── Threads ─────────────────────────────────────────────────────

// GetThread returns all posts in a thread (root + replies, oldest first).
func (c *Client) GetThread(ctx context.Context, rootID string) ([]*model.Post, error) {
	var list model.PostList
	if err := c.do(ctx, http.MethodGet, "/api/v1/posts/"+rootID+"/thread", nil, &list); err != nil {
		return nil, err
	}
	return list.Order, nil
}

// GetMyThreads returns threads the agent follows in a team.
func (c *Client) GetMyThreads(ctx context.Context, teamID string, page, perPage int) (*model.UserThreadList, error) {
	var threads model.UserThreadList
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me/teams/"+teamID+"/threads"+pageQuery(page, perPage), nil, &threads); err != nil {
		return nil, err
	}
	return &threads, nil
}

// MarkThreadAsRead marks a thread as read for the agent.
func (c *Client) MarkThreadAsRead(ctx context.Context, teamID, postID string) error {
	return c.do(ctx, http.MethodPut, "/api/v1/users/me/teams/"+teamID+"/threads/"+postID+"/read", nil, nil)
}

// UpdateThreadFollowing follows or unfollows a thread for the agent.
func (c *Client) UpdateThreadFollowing(ctx context.Context, teamID, postID string, following bool) error {
	body := map[string]bool{"following": following}
	return c.do(ctx, http.MethodPut, "/api/v1/users/me/teams/"+teamID+"/threads/"+postID+"/following", body, nil)
}

// ─── Tags ────────────────────────────────────────────────────────

// GetAllTags lists all tags.
func (c *Client) GetAllTags(ctx context.Context) ([]*model.Tag, error) {
	var tags []*model.Tag
	if err := c.do(ctx, http.MethodGet, "/api/v1/tags", nil, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// AddTagToPost tags a post.
func (c *Client) AddTagToPost(ctx context.Context, postID, tagID string) error {
	body := map[string]string{"tag_id": tagID}
	return c.do(ctx, http.MethodPost, "/api/v1/posts/"+postID+"/tags", body, nil)
}

// GetTagsForPost returns a post's tags.
func (c *Client) GetTagsForPost(ctx context.Context, postID string) ([]*model.Tag, error) {
	var tags []*model.Tag
	if err := c.do(ctx, http.MethodGet, "/api/v1/posts/"+postID+"/tags", nil, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// ─── WebSocket ───────────────────────────────────────────────────

// Listen connects to chitd's WebSocket and invokes handle for every event
// until ctx is cancelled, reconnecting with backoff on connection loss.
func (c *Client) Listen(ctx context.Context, handle func(*model.WebSocketEvent)) {
	wsURL := strings.Replace(c.baseURL, "http", "ws", 1) + "/api/v1/websocket"
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		headers, err := c.authHeaders()
		if err != nil {
			// A token fetch failure is usually transient (Hydra restarting,
			// clock skew); back off and retry rather than exiting the loop.
			slog.Info("websocket auth failed, retrying", "error", err, "backoff", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}

		conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			slog.Info("websocket connect failed, retrying", "error", err, "backoff", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		slog.Info("websocket connected", "url", wsURL)

		// Close the connection when ctx is cancelled so ReadMessage unblocks.
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = conn.Close()
			case <-done:
			}
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if ctx.Err() == nil {
					slog.Info("websocket read failed, reconnecting", "error", err)
				}
				break
			}
			var event model.WebSocketEvent
			if err := json.Unmarshal(message, &event); err != nil {
				slog.Info("invalid websocket event", "error", err)
				continue
			}
			handle(&event)
		}
		close(done)
		_ = conn.Close()
	}
}
