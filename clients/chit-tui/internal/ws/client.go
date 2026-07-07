package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/infrashift/chit-tui/internal/model"
)

// WSClient defines the WebSocket interface for the TUI.
type WSClient interface {
	Connect() error
	Close() error
	Events() <-chan model.WebSocketEvent
	Send(msg model.WebSocketMessage) error
	SetToken(token string)
}

type wsClient struct {
	url        string
	token      string
	headerName string
	conn       *websocket.Conn
	events     chan model.WebSocketEvent
	done       chan struct{}
	backoff    *Backoff
	mu         sync.Mutex
}

// NewWSClient creates a new WebSocket client.
func NewWSClient(url, token string, bufSize int) WSClient {
	return NewWSClientWithHeader(url, token, bufSize, "X-Session-Token")
}

// NewWSClientWithHeader creates a WebSocket client that uses a custom auth header name.
func NewWSClientWithHeader(url, token string, bufSize int, headerName string) WSClient {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &wsClient{
		url:        url,
		token:      token,
		headerName: headerName,
		events:     make(chan model.WebSocketEvent, bufSize),
		done:       make(chan struct{}),
		backoff:    NewBackoff(),
	}
}

func (c *wsClient) Connect() error {
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()

	header := http.Header{}
	header.Set(c.headerName, token)

	conn, resp, err := websocket.DefaultDialer.Dial(c.url, header)
	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
			return fmt.Errorf("ws: %w (HTTP %d)", err, resp.StatusCode)
		}
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	go c.readLoop()
	return nil
}

func (c *wsClient) readLoop() {
	for {
		select {
		case <-c.done:
			c.closeConn()
			return
		default:
		}

		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			c.closeConn()
			if !c.reconnect() {
				return
			}
			continue
		}

		var evt model.WebSocketEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			continue
		}

		select {
		case c.events <- evt:
		case <-c.done:
			c.closeConn()
			return
		default:
			// Drop event if buffer is full.
		}
	}
}

func (c *wsClient) closeConn() {
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
}

func (c *wsClient) reconnect() bool {
	for {
		wait := c.backoff.Next()

		select {
		case <-c.done:
			return false
		case <-time.After(wait):
		}

		c.mu.Lock()
		token := c.token
		c.mu.Unlock()

		header := http.Header{}
		header.Set(c.headerName, token)

		conn, _, err := websocket.DefaultDialer.Dial(c.url, header)
		if err != nil {
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()
		c.backoff.Reset()
		return true
	}
}

func (c *wsClient) Close() error {
	close(c.done)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// SetToken updates the token used for reconnections.
func (c *wsClient) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

func (c *wsClient) Events() <-chan model.WebSocketEvent {
	return c.events
}

func (c *wsClient) Send(msg model.WebSocketMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return ErrNotConnected
	}
	return c.conn.WriteJSON(msg)
}
