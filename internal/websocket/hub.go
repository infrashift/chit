package websocket

import (
	"log/slog"

	"github.com/infrashift/chit/internal/model"
)

const sendBufferSize = 256

// MembershipChecker supplies channel memberships so the hub can scope
// channel-targeted events to members. Defined here (not in store) to avoid an
// import cycle: app depends on websocket.
type MembershipChecker interface {
	GetChannelIDsForUser(userID string) ([]string, error)
}

type membershipLoad struct {
	userID   string
	channels map[string]struct{}
	ok       bool
}

type membershipChange struct {
	userID    string
	channelID string
	added     bool
}

// Hub manages per-user WebSocket connections and event broadcasting.
// All map operations are serialized through a single event loop goroutine.
type Hub struct {
	checker MembershipChecker

	clients     map[string][]*Client           // userID → connections
	memberships map[string]map[string]struct{} // userID → channel set
	loading     map[string]bool                // userID → membership load in flight

	register         chan *Client
	unregister       chan *Client
	broadcast        chan *model.WebSocketEvent
	membershipLoaded chan membershipLoad
	membershipChange chan membershipChange
	stop             chan struct{}
	done             chan struct{}
}

// NewHub creates a new WebSocket Hub and starts its event loop.
// A nil checker disables membership filtering: channel-targeted events are
// then delivered to every connected user (only safe when no untrusted clients
// connect, e.g. the MCP server's internal hub).
func NewHub(checker MembershipChecker) *Hub {
	h := &Hub{
		checker:          checker,
		clients:          make(map[string][]*Client),
		memberships:      make(map[string]map[string]struct{}),
		loading:          make(map[string]bool),
		register:         make(chan *Client),
		unregister:       make(chan *Client),
		broadcast:        make(chan *model.WebSocketEvent, 256),
		membershipLoaded: make(chan membershipLoad),
		membershipChange: make(chan membershipChange),
		stop:             make(chan struct{}),
		done:             make(chan struct{}),
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
		case load := <-h.membershipLoaded:
			h.applyMembershipLoad(load)
		case change := <-h.membershipChange:
			h.applyMembershipChange(change)
		case <-h.stop:
			h.closeAll()
			return
		}
	}
}

func (h *Hub) addClient(client *Client) {
	h.clients[client.UserID] = append(h.clients[client.UserID], client)
	h.loadMemberships(client.UserID)
	slog.Debug("websocket: client registered", "user_id", client.UserID)
}

// loadMemberships starts an async membership load for the user's first
// connection. Channel-targeted events are dropped for the user until the load
// completes.
func (h *Hub) loadMemberships(userID string) {
	if h.checker == nil || h.loading[userID] {
		return
	}
	if _, ok := h.memberships[userID]; ok {
		return
	}
	h.loading[userID] = true

	go func() {
		load := membershipLoad{userID: userID, channels: make(map[string]struct{})}
		channelIDs, err := h.checker.GetChannelIDsForUser(userID)
		if err != nil {
			slog.Error("websocket: failed to load channel memberships", "user_id", userID, "error", err)
		} else {
			load.ok = true
			for _, id := range channelIDs {
				load.channels[id] = struct{}{}
			}
		}
		select {
		case h.membershipLoaded <- load:
		case <-h.stop:
		}
	}()
}

func (h *Hub) applyMembershipLoad(load membershipLoad) {
	delete(h.loading, load.userID)
	if !load.ok {
		return // next register retries the load
	}
	if _, connected := h.clients[load.userID]; !connected {
		return // user disconnected while loading
	}
	h.memberships[load.userID] = load.channels
}

func (h *Hub) applyMembershipChange(change membershipChange) {
	channels, ok := h.memberships[change.userID]
	if !ok {
		return // not connected (or load still in flight — the load will observe the change)
	}
	if change.added {
		channels[change.channelID] = struct{}{}
	} else {
		delete(channels, change.channelID)
	}
}

// removeClient removes a client and closes its send channel. It is idempotent:
// a client already removed (e.g. slow-consumer disconnect racing the readPump's
// deferred Unregister) is ignored.
func (h *Hub) removeClient(client *Client) {
	clients := h.clients[client.UserID]
	for i, c := range clients {
		if c != client {
			continue
		}
		h.clients[client.UserID] = append(clients[:i], clients[i+1:]...)
		if len(h.clients[client.UserID]) == 0 {
			delete(h.clients, client.UserID)
			delete(h.memberships, client.UserID)
		}
		close(client.send)
		slog.Debug("websocket: client unregistered", "user_id", client.UserID)
		return
	}
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
				// Slow consumer — disconnect. We are inside the event loop, so
				// remove inline; closing the conn unwinds the pumps, and the
				// readPump's deferred Unregister becomes a harmless no-op.
				slog.Warn("websocket: slow consumer, disconnecting", "user_id", userID)
				h.removeClient(client)
				if client.conn != nil {
					_ = client.conn.Close()
				}
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

	// Channel-targeted events go only to channel members. Events are dropped
	// for users whose membership load has not completed yet.
	if bc.ChannelID != "" && h.checker != nil {
		channels, ok := h.memberships[userID]
		if !ok {
			return false
		}
		_, member := channels[bc.ChannelID]
		return member
	}

	// Team-targeted events (channel_created etc.) remain broadcast to all
	// connected users.
	return true
}

func (h *Hub) closeAll() {
	for _, clients := range h.clients {
		for _, client := range clients {
			close(client.send)
		}
	}
	h.clients = make(map[string][]*Client)
	h.memberships = make(map[string]map[string]struct{})
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

// Broadcast sends an event to applicable clients.
func (h *Hub) Broadcast(event *model.WebSocketEvent) {
	h.broadcast <- event
}

// NotifyMembershipChanged keeps the hub's membership cache current when users
// join or leave channels.
func (h *Hub) NotifyMembershipChanged(userID, channelID string, added bool) {
	select {
	case h.membershipChange <- membershipChange{userID: userID, channelID: channelID, added: added}:
	case <-h.done:
	}
}

// Stop shuts down the hub event loop and closes all connections.
func (h *Hub) Stop() {
	close(h.stop)
	<-h.done
}
