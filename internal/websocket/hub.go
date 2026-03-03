package websocket

import (
	"log/slog"
	"sync"

	"github.com/infrashift/chit/internal/model"
)

const sendBufferSize = 256

// Hub manages per-user WebSocket connections and event broadcasting.
// All map operations are serialized through a single event loop goroutine.
type Hub struct {
	clients    map[string][]*Client // userID → connections
	register   chan *Client
	unregister chan *Client
	broadcast  chan *model.WebSocketEvent
	stop       chan struct{}
	done       chan struct{}
	mu         sync.RWMutex // protects reads for connection count, etc.
}

// NewHub creates a new WebSocket Hub and starts its event loop.
func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[string][]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *model.WebSocketEvent, 256),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	defer close(h.done)

	for {
		select {
		case client := <-h.register:
			h.addClient(client)
		case client := <-h.unregister:
			h.removeClient(client)
		case event := <-h.broadcast:
			h.broadcastEvent(event)
		case <-h.stop:
			h.closeAll()
			return
		}
	}
}

func (h *Hub) addClient(client *Client) {
	h.clients[client.UserID] = append(h.clients[client.UserID], client)
	slog.Debug("websocket: client registered", "user_id", client.UserID)
}

func (h *Hub) removeClient(client *Client) {
	clients := h.clients[client.UserID]
	for i, c := range clients {
		if c == client {
			h.clients[client.UserID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(h.clients[client.UserID]) == 0 {
		delete(h.clients, client.UserID)
	}
	close(client.send)
	slog.Debug("websocket: client unregistered", "user_id", client.UserID)
}

func (h *Hub) broadcastEvent(event *model.WebSocketEvent) {
	if event.Broadcast == nil {
		return
	}

	for userID, clients := range h.clients {
		if !h.shouldSend(event, userID) {
			continue
		}
		for _, client := range clients {
			select {
			case client.send <- event:
			default:
				// Slow consumer — disconnect
				slog.Warn("websocket: slow consumer, disconnecting", "user_id", userID)
				go func(c *Client) { h.unregister <- c }(client)
			}
		}
	}
}

func (h *Hub) shouldSend(event *model.WebSocketEvent, userID string) bool {
	bc := event.Broadcast

	// If targeting a specific user, only send to that user
	if bc.UserID != "" {
		return bc.UserID == userID
	}

	// Channel and team filtering would require membership lookups;
	// for now, broadcast to all connected users and let the client filter.
	// TODO: add channel membership check via store or in-memory cache
	return true
}

func (h *Hub) closeAll() {
	for _, clients := range h.clients {
		for _, client := range clients {
			close(client.send)
		}
	}
	h.clients = make(map[string][]*Client)
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast sends an event to applicable clients.
func (h *Hub) Broadcast(event *model.WebSocketEvent) {
	h.broadcast <- event
}

// Stop shuts down the hub event loop and closes all connections.
func (h *Hub) Stop() {
	close(h.stop)
	<-h.done
}
