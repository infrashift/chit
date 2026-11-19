package websocket

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/errlog"
	"github.com/infrashift/chit/internal/model"
)

// Client represents a single WebSocket connection for a user.
type Client struct {
	UserID string
	conn   *websocket.Conn
	hub    *Hub
	send   chan *model.WebSocketEvent

	pingInterval time.Duration
	writeTimeout time.Duration
}

// NewClient creates a WebSocket Client. Register it with the hub, then call
// Start.
func NewClient(hub *Hub, conn *websocket.Conn, userID string, pingInterval, writeTimeout time.Duration) *Client {
	assert.That(hub != nil, "NewClient: nil hub")
	assert.That(conn != nil, "NewClient: nil connection")
	assert.That(pingInterval > 0, "NewClient: ping interval not positive")
	assert.That(writeTimeout > 0, "NewClient: write timeout not positive")
	return &Client{
		UserID:       userID,
		conn:         conn,
		hub:          hub,
		send:         make(chan *model.WebSocketEvent, sendBufferSize),
		pingInterval: pingInterval,
		writeTimeout: writeTimeout,
	}
}

// Start runs the client's read and write pumps. Call it after Hub.Register:
// if the pumps started first and the peer hung up at once, the readPump's
// Unregister would land before the Register, and the hub would then keep a
// dead client until its send buffer filled.
func (c *Client) Start() {
	assert.That(c.conn != nil, "Client.Start: nil connection")
	assert.That(c.hub != nil, "Client.Start: nil hub")
	assert.That(c.send != nil, "Client.Start: nil send channel")
	go c.writePump()
	go c.readPump()
}

func (c *Client) readPump() {
	defer errlog.Recover("websocket read pump")
	assert.That(c.conn != nil, "Client.readPump: nil connection")
	assert.That(c.hub != nil, "Client.readPump: nil hub")
	assert.That(c.pingInterval > 0, "Client.readPump: ping interval not positive")
	defer func() {
		c.hub.Unregister(c)
		errlog.Close(c.conn, "websocket connection (read pump)")
	}()

	c.conn.SetReadLimit(4096)
	// A failed deadline means the conn is already broken; the next read
	// returns that error and ends the pump.
	errlog.Debug(c.conn.SetReadDeadline(time.Now().Add(c.pingInterval*2)), "websocket set read deadline")
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.pingInterval * 2))
	})

	// The loop ends when a read fails. The read deadline above makes a
	// silent peer fail, so the loop has a bound (Power of 10 rule 2).
	_, message, err := c.conn.ReadMessage()
	for ; err == nil; _, message, err = c.conn.ReadMessage() {
		var wsMsg model.WebSocketMessage
		if decodeErr := json.Unmarshal(message, &wsMsg); decodeErr != nil {
			slog.Warn("websocket: invalid message", "user_id", c.UserID, "error", decodeErr)
			continue
		}

		// Handle client messages (e.g., typing indicators)
		c.handleMessage(&wsMsg)
	}
	if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
		slog.Warn("websocket: unexpected close", "user_id", c.UserID, "error", err)
	}
}

func (c *Client) writePump() {
	defer errlog.Recover("websocket write pump")
	assert.That(c.conn != nil, "Client.writePump: nil connection")
	assert.That(c.pingInterval > 0, "Client.writePump: ping interval not positive")
	ticker := time.NewTicker(c.pingInterval)
	defer func() {
		ticker.Stop()
		errlog.Close(c.conn, "websocket connection (write pump)")
	}()

	// The loop ends when the hub closes send or a write fails. The write
	// deadline makes a stuck peer fail (Power of 10 rule 2).
	open := true
	for open {
		select {
		case event, ok := <-c.send:
			open = c.writeEvent(event, ok)
		case <-ticker.C:
			open = c.writePing()
		}
	}
}

// writeEvent writes one event from send. ok is false when the hub closed
// send. It reports whether the pump goes on.
func (c *Client) writeEvent(event *model.WebSocketEvent, ok bool) bool {
	assert.That(c.conn != nil, "Client.writeEvent: nil connection")
	assert.That(ok || event == nil, "Client.writeEvent: event from a closed send channel")
	assert.That(!ok || event != nil, "Client.writeEvent: hub queued a nil event")
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
		return false
	}
	if !ok {
		// The hub closed send; tell the peer, best effort.
		errlog.Debug(c.conn.WriteMessage(websocket.CloseMessage, []byte{}), "websocket write close message")
		return false
	}
	if err := c.conn.WriteJSON(event); err != nil {
		slog.Warn("websocket: write error", "user_id", c.UserID, "error", err)
		return false
	}
	return true
}

// writePing sends a ping frame. It reports whether the pump goes on.
func (c *Client) writePing() bool {
	assert.That(c.conn != nil, "Client.writePing: nil connection")
	assert.That(c.writeTimeout > 0, "Client.writePing: write timeout not positive")
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
		return false
	}
	return c.conn.WriteMessage(websocket.PingMessage, nil) == nil
}

func (c *Client) handleMessage(msg *model.WebSocketMessage) {
	assert.That(msg != nil, "Client.handleMessage: nil message")
	assert.That(c.hub != nil, "Client.handleMessage: nil hub")
	// Typing is the only action a client may send; anything else is ignored.
	if msg.Action != "typing" {
		return
	}
	channelID, ok := msg.Data["channel_id"].(string)
	if !ok {
		return
	}
	c.hub.BroadcastFromUser(c.UserID, &model.WebSocketEvent{
		Event: model.WebSocketEventTyping,
		Data: map[string]any{
			"user_id": c.UserID,
		},
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: channelID,
		},
	})
}
