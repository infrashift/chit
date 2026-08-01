package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// ConnState reports a transport-level transition. It is delivered on its own
// channel rather than as a synthetic event so that server events and
// connection state never have to be told apart downstream.
type ConnState struct {
	// Connected is the state now being entered.
	Connected bool
	// Err explains a disconnect, when one is known.
	Err error
	// Unauthorized marks a failure the client cannot recover from by
	// retrying: the credentials were rejected, so reconnecting forever would
	// only hide the need to sign in again.
	Unauthorized bool
}

// WSClient defines the WebSocket interface for the TUI.
type WSClient interface {
	Connect() error
	Close() error
	Events() <-chan model.WebSocketEvent
	// State reports connect and disconnect transitions.
	State() <-chan ConnState
	Send(msg model.WebSocketMessage) error
	SetToken(token string)
}

type wsClient struct {
	url        string
	token      string
	headerName string
	conn       *websocket.Conn
	events     chan model.WebSocketEvent
	state      chan ConnState
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
		// Small buffer: transitions are rare and only the latest matters, so
		// dropping one under contention is preferable to blocking the reader.
		state:   make(chan ConnState, 8),
		done:    make(chan struct{}),
		backoff: NewBackoff(),
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

	c.emitState(ConnState{Connected: true})

	go c.readLoop()
	return nil
}

// State returns the connection-state channel.
func (c *wsClient) State() <-chan ConnState { return c.state }

// emitState publishes a transition without ever blocking: the reader may be
// busy, and a stalled read loop would stop reconnecting altogether.
func (c *wsClient) emitState(s ConnState) {
	select {
	case c.state <- s:
	default:
	}
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
			c.emitState(ConnState{Connected: false, Err: err})
			if !c.reconnect() {
				return
			}
			c.emitState(ConnState{Connected: true})
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

		conn, resp, err := websocket.DefaultDialer.Dial(c.url, header)
		if err != nil {
			// Retrying cannot fix rejected credentials. Report it and stop,
			// so the UI can ask the user to sign in again instead of the
			// loop dialing forever behind a dead session.
			if resp != nil {
				status := resp.StatusCode
				_ = resp.Body.Close()
				if status == http.StatusUnauthorized || status == http.StatusForbidden {
					c.emitState(ConnState{Connected: false, Err: err, Unauthorized: true})
					return false
				}
			}
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
