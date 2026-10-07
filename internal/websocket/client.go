package websocket

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"

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

// NewClient creates a new WebSocket Client and starts its read/write pumps.
func NewClient(hub *Hub, conn *websocket.Conn, userID string, pingInterval, writeTimeout time.Duration) *Client {
	c := &Client{
		UserID:       userID,
		conn:         conn,
		hub:          hub,
		send:         make(chan *model.WebSocketEvent, sendBufferSize),
		pingInterval: pingInterval,
		writeTimeout: writeTimeout,
	}
	go c.writePump()
	go c.readPump()
	return c
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	// A failed deadline means the conn is already broken; the next read
	// returns that error and ends the pump.
	_ = c.conn.SetReadDeadline(time.Now().Add(c.pingInterval * 2))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.pingInterval * 2))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("websocket: unexpected close", "user_id", c.UserID, "error", err)
			}
			return
		}

		var wsMsg model.WebSocketMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			slog.Warn("websocket: invalid message", "user_id", c.UserID, "error", err)
			continue
		}

		// Handle client messages (e.g., typing indicators)
		c.handleMessage(&wsMsg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(c.pingInterval)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
				return
			}
			if !ok {
				// The hub closed send; tell the peer, best effort.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(event); err != nil {
				slog.Warn("websocket: write error", "user_id", c.UserID, "error", err)
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg *model.WebSocketMessage) {
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
