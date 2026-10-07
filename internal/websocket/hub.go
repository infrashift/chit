package websocket

import (
	"log/slog"

	"github.com/infrashift/chit/internal/model"
)

const sendBufferSize = 256

// MembershipChecker supplies channel and team memberships so the hub can
// scope targeted events to members. Defined here (not in store) to avoid an
// import cycle: app depends on websocket.
type MembershipChecker interface {
	GetChannelIDsForUser(userID string) ([]string, error)
	GetTeamIDsForUser(userID string) ([]string, error)
}

// memberships holds one user's channel and team sets, loaded atomically so a
// partially failed load never filters against half the data.
type memberships struct {
	channels map[string]struct{}
	teams    map[string]struct{}
}

type membershipLoad struct {
	userID string
	sets   memberships
	ok     bool
}

type membershipChange struct {
	userID string
	id     string // channel or team ID, per team flag
	team   bool
	added  bool
}

// broadcastRequest pairs an event with the connected user that produced it.
// senderID is empty for trusted server-side events; when set, channel-targeted
// events are dropped unless the sender is a member of the target channel.
type broadcastRequest struct {
	event    *model.WebSocketEvent
	senderID string
}

// Hub manages per-user WebSocket connections and event broadcasting.
// All map operations are serialized through a single event loop goroutine.
type Hub struct {
	checker MembershipChecker

	clients     map[string][]*Client   // userID → connections
	memberships map[string]memberships // userID → channel/team sets
	loading     map[string]bool        // userID → membership load in flight
	// pending holds membership changes that arrived while the user's load was
	// in flight. The load may already have read the store, so they are
	// replayed on top of its result rather than dropped.
	pending map[string][]membershipChange

	register         chan *Client
	unregister       chan *Client
	broadcast        chan broadcastRequest
	membershipLoaded chan membershipLoad
	membershipChange chan membershipChange
	stop             chan struct{}
	done             chan struct{}
}

// NewHub creates a new WebSocket Hub and starts its event loop.
// A nil checker disables membership filtering: channel- and team-targeted
// events are then delivered to every connected user (only safe when no
// untrusted clients connect, e.g. the MCP server's internal hub).
func NewHub(checker MembershipChecker) *Hub {
	h := &Hub{
		checker:          checker,
		clients:          make(map[string][]*Client),
		memberships:      make(map[string]memberships),
		loading:          make(map[string]bool),
		pending:          make(map[string][]membershipChange),
		register:         make(chan *Client),
		unregister:       make(chan *Client),
		broadcast:        make(chan broadcastRequest, 256),
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
		case req := <-h.broadcast:
			h.broadcastEvent(req)
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
// connection. Channel- and team-targeted events are dropped for the user
// until the load completes.
func (h *Hub) loadMemberships(userID string) {
	if h.checker == nil || h.loading[userID] {
		return
	}
	if _, ok := h.memberships[userID]; ok {
		return
	}
	h.loading[userID] = true

	go func() {
		load := membershipLoad{userID: userID, sets: memberships{
			channels: make(map[string]struct{}),
			teams:    make(map[string]struct{}),
		}}
		channelIDs, err := h.checker.GetChannelIDsForUser(userID)
		if err == nil {
			var teamIDs []string
			teamIDs, err = h.checker.GetTeamIDsForUser(userID)
			if err == nil {
				load.ok = true
				for _, id := range channelIDs {
					load.sets.channels[id] = struct{}{}
				}
				for _, id := range teamIDs {
					load.sets.teams[id] = struct{}{}
				}
			}
		}
		if err != nil {
			slog.Error("websocket: failed to load memberships", "user_id", userID, "error", err)
		}
		select {
		case h.membershipLoaded <- load:
		case <-h.stop:
		}
	}()
}

func (h *Hub) applyMembershipLoad(load membershipLoad) {
	delete(h.loading, load.userID)
	pending := h.pending[load.userID]
	delete(h.pending, load.userID)
	if !load.ok {
		return // next register retries the load, which re-reads everything
	}
	if _, connected := h.clients[load.userID]; !connected {
		return // user disconnected while loading
	}
	h.memberships[load.userID] = load.sets
	for _, change := range pending {
		h.applyMembershipChange(change)
	}
}

func (h *Hub) applyMembershipChange(change membershipChange) {
	sets, ok := h.memberships[change.userID]
	if !ok {
		if h.loading[change.userID] {
			h.pending[change.userID] = append(h.pending[change.userID], change)
		}
		return // not connected: the next load reads current membership
	}
	set := sets.channels
	if change.team {
		set = sets.teams
	}
	if change.added {
		set[change.id] = struct{}{}
	} else {
		delete(set, change.id)
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
			delete(h.pending, client.UserID)
		}
		close(client.send)
		slog.Debug("websocket: client unregistered", "user_id", client.UserID)
		return
	}
}

func (h *Hub) broadcastEvent(req broadcastRequest) {
	event := req.event
	if event.Broadcast == nil {
		return
	}
	if !h.senderAllowed(req) {
		slog.Warn("websocket: dropping client event for non-member sender",
			"user_id", req.senderID, "event", event.Event, "channel_id", event.Broadcast.ChannelID)
		return
	}

	// Slow consumers are collected and removed after the loop. Removing one
	// inline shifts its user's slice while this loop is ranging over it, so
	// the loop meets a later client twice, the second time after its send
	// channel is closed: a send on a closed channel panics the hub goroutine,
	// and with it the process.
	var slow []*Client
	for userID, clients := range h.clients {
		if !h.shouldSend(event, userID) {
			continue
		}
		for _, client := range clients {
			select {
			case client.send <- event:
			default:
				slow = append(slow, client)
			}
		}
	}
	for _, client := range slow {
		// Closing the conn unwinds the pumps, and the readPump's deferred
		// Unregister becomes a harmless no-op.
		slog.Warn("websocket: slow consumer, disconnecting", "user_id", client.UserID)
		h.removeClient(client)
		if client.conn != nil {
			_ = client.conn.Close()
		}
	}
}

// senderAllowed reports whether a client-originated event (senderID set) may
// be broadcast: channel-targeted events require the sender to be a member of
// the target channel. Server-side events (empty senderID) are always allowed,
// as is everything on a hub without a checker.
func (h *Hub) senderAllowed(req broadcastRequest) bool {
	if req.senderID == "" || h.checker == nil || req.event.Broadcast.ChannelID == "" {
		return true
	}
	sets, ok := h.memberships[req.senderID]
	if !ok {
		return false // membership load not completed — same policy as delivery
	}
	_, member := sets.channels[req.event.Broadcast.ChannelID]
	return member
}

func (h *Hub) shouldSend(event *model.WebSocketEvent, userID string) bool {
	bc := event.Broadcast

	// If targeting a specific user, only send to that user
	if bc.UserID != "" {
		return bc.UserID == userID
	}

	// Channel- and team-targeted events go only to members. Events are
	// dropped for users whose membership load has not completed yet.
	if bc.ChannelID != "" && h.checker != nil {
		sets, ok := h.memberships[userID]
		if !ok {
			return false
		}
		_, member := sets.channels[bc.ChannelID]
		return member
	}
	if bc.TeamID != "" && h.checker != nil {
		sets, ok := h.memberships[userID]
		if !ok {
			return false
		}
		_, member := sets.teams[bc.TeamID]
		return member
	}

	// Target-less events remain broadcast to all connected users.
	return true
}

func (h *Hub) closeAll() {
	for _, clients := range h.clients {
		for _, client := range clients {
			close(client.send)
		}
	}
	h.clients = make(map[string][]*Client)
	h.memberships = make(map[string]memberships)
}

// Register adds a client to the hub. After Stop it closes the client's send
// channel instead, so a connection accepted during shutdown is told to go.
func (h *Hub) Register(client *Client) {
	select {
	case h.register <- client:
	case <-h.done:
		close(client.send)
	}
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

// Broadcast sends a trusted server-side event to applicable clients. After
// Stop it is a no-op.
func (h *Hub) Broadcast(event *model.WebSocketEvent) {
	h.send(broadcastRequest{event: event})
}

// BroadcastFromUser sends a client-originated event (e.g. typing). Channel-
// targeted events are dropped unless the sender is a member of the channel.
func (h *Hub) BroadcastFromUser(senderID string, event *model.WebSocketEvent) {
	h.send(broadcastRequest{event: event, senderID: senderID})
}

// send queues req, or drops it once the hub has stopped: nothing drains the
// queue then, and blocking would hang the request that published the event.
func (h *Hub) send(req broadcastRequest) {
	select {
	case h.broadcast <- req:
	case <-h.done:
	}
}

// NotifyMembershipChanged keeps the hub's membership cache current when users
// join or leave channels.
func (h *Hub) NotifyMembershipChanged(userID, channelID string, added bool) {
	select {
	case h.membershipChange <- membershipChange{userID: userID, id: channelID, added: added}:
	case <-h.done:
	}
}

// NotifyTeamMembershipChanged keeps the hub's membership cache current when
// users join or leave teams.
func (h *Hub) NotifyTeamMembershipChanged(userID, teamID string, added bool) {
	select {
	case h.membershipChange <- membershipChange{userID: userID, id: teamID, team: true, added: added}:
	case <-h.done:
	}
}

// Stop shuts down the hub event loop and closes all connections.
func (h *Hub) Stop() {
	close(h.stop)
	<-h.done
}
