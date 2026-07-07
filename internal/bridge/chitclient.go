package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/infrashift/chit/internal/model"
)

// ChitClient talks to chitd over its HTTP API and WebSocket, authenticating
// with the agent's Kratos ID via the trusted proxy header. It connects to
// chitd directly (not through Oathkeeper), the same trust position Oathkeeper
// itself holds; set ProxySecret when chitd requires X-Proxy-Secret.
type ChitClient struct {
	baseURL     string
	kratosID    string
	proxySecret string
	http        *http.Client
}

// NewChitClient creates a client for the given chitd base URL.
func NewChitClient(baseURL, kratosID, proxySecret string) *ChitClient {
	return &ChitClient{
		baseURL:     strings.TrimRight(baseURL, "/"),
		kratosID:    kratosID,
		proxySecret: proxySecret,
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *ChitClient) authHeaders() http.Header {
	h := http.Header{}
	h.Set("X-User-Id", c.kratosID)
	if c.proxySecret != "" {
		h.Set("X-Proxy-Secret", c.proxySecret)
	}
	return h
}

func (c *ChitClient) do(ctx context.Context, method, path string, body, out any) error {
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
	for k, vs := range c.authHeaders() {
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
	defer resp.Body.Close()

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

// Me returns the agent's own user record (provisioning it on first contact).
func (c *ChitClient) Me(ctx context.Context) (*model.User, error) {
	var user model.User
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// CreatePost posts a message as the agent. rootID may be empty for a root post.
func (c *ChitClient) CreatePost(ctx context.Context, channelID, rootID, content string, props map[string]any) (*model.Post, error) {
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

// GetThread returns all posts in a thread (root + replies, oldest first).
func (c *ChitClient) GetThread(ctx context.Context, rootID string) ([]*model.Post, error) {
	var list model.PostList
	if err := c.do(ctx, http.MethodGet, "/api/v1/posts/"+rootID+"/thread", nil, &list); err != nil {
		return nil, err
	}
	return list.Order, nil
}

// Listen connects to chitd's WebSocket and invokes handle for every event
// until ctx is cancelled, reconnecting with backoff on connection loss.
func (c *ChitClient) Listen(ctx context.Context, handle func(*model.WebSocketEvent)) {
	wsURL := strings.Replace(c.baseURL, "http", "ws", 1) + "/api/v1/websocket"
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, c.authHeaders())
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			logf("websocket connect failed, retrying", "error", err, "backoff", backoff)
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
		logf("websocket connected", "url", wsURL)

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
					logf("websocket read failed, reconnecting", "error", err)
				}
				break
			}
			var event model.WebSocketEvent
			if err := json.Unmarshal(message, &event); err != nil {
				logf("invalid websocket event", "error", err)
				continue
			}
			handle(&event)
		}
		close(done)
		_ = conn.Close()
	}
}
