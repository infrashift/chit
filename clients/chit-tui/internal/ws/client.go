package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
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
	// Desynced reports that events were discarded because the reader fell
	// behind. The socket is still up, so nothing else would reveal it — the
	// view would simply stop matching the server. Callers should resync the
	// same way they do after a reconnect.
	Desynced bool
}

// WSClient defines the WebSocket interface for the TUI.
type WSClient interface {
	Connect() error
	Close() error
	Events() <-chan model.WebSocketEvent
	// State reports connect and disconnect transitions.
	State() <-chan ConnState
	SetToken(token string)
}

type wsClient struct {
	url        string
	headerName string
	events     chan model.WebSocketEvent
	state      chan ConnState

	// backoffBase and backoffMax set the delay schedule for one session's
	// redials; newBackoff builds it.
	backoffBase time.Duration
	backoffMax  time.Duration
	// pongWait is how long the link may stay silent before it is declared
	// dead; pingPeriod is how often a ping is sent to break that silence.
	pongWait   time.Duration
	pingPeriod time.Duration

	mu    sync.Mutex
	token string
	// conn is the live connection, if any. Only the session's run loop
	// reads from it; Close reaches it through mu.
	conn *websocket.Conn
	// stop is closed to end the current session. It is nil while closed,
	// which is what lets Close be called twice and Connect be called again.
	stop chan struct{}
}

const (
	defaultPongWait   = 60 * time.Second
	defaultPingPeriod = 25 * time.Second
	writeWait         = 10 * time.Second
)

// NewWSClient creates a new WebSocket client.
func NewWSClient(url, token string, bufSize int) WSClient {
	return NewWSClientWithHeader(url, token, bufSize, "X-Session-Token")
}

// NewWSClientWithHeader creates a WebSocket client that uses a custom auth header name.
func NewWSClientWithHeader(url, token string, bufSize int, headerName string) WSClient {
	assert.That(url != "", "NewWSClientWithHeader: empty URL")
	assert.That(headerName != "", "NewWSClientWithHeader: empty header name")
	if bufSize <= 0 {
		bufSize = 256
	}
	assert.That(bufSize > 0, "NewWSClientWithHeader: event buffer must have room")
	c := &wsClient{
		url:        url,
		token:      token,
		headerName: headerName,
		events:     make(chan model.WebSocketEvent, bufSize),
		// Small buffer: transitions are rare and only the latest matters, so
		// dropping one under contention is preferable to blocking the reader.
		state:       make(chan ConnState, 8),
		backoffBase: defaultBackoffBase,
		backoffMax:  defaultBackoffMax,
		pongWait:    defaultPongWait,
		pingPeriod:  defaultPingPeriod,
	}
	assert.That(c.pingPeriod < c.pongWait, "NewWSClientWithHeader: a ping must come before the link is declared dead")
	return c
}

// newBackoff builds a fresh delay schedule for one session's redials.
func (c *wsClient) newBackoff() *Backoff {
	assert.That(c.backoffBase > 0, "newBackoff: backoff base must be positive")
	assert.That(c.backoffMax >= c.backoffBase, "newBackoff: backoff max is below its base")
	return &Backoff{Base: c.backoffBase, Max: c.backoffMax}
}

// Connect starts a session. A failed first dial is returned so the caller
// can show it, but unless the credentials were rejected the session keeps
// redialing in the background, the same as after a drop. Calling Connect on
// a running session does nothing.
func (c *wsClient) Connect() error {
	assert.That(c.events != nil && c.state != nil, "Connect: client not built by NewWSClient")
	c.mu.Lock()
	if c.stop != nil {
		c.mu.Unlock()
		return nil
	}
	assert.That(c.conn == nil, "Connect: a connection outlived its session")
	stop := make(chan struct{})
	c.stop = stop
	c.mu.Unlock()

	conn, err := c.dial()
	if err != nil {
		assert.That(conn == nil, "Connect: a failed dial returned a connection")
		if errors.Is(err, ErrUnauthorized) {
			c.mu.Lock()
			if c.stop == stop {
				c.stop = nil
			}
			c.mu.Unlock()
			return err
		}
		go c.run(stop, nil)
		return err
	}
	assert.That(conn != nil, "Connect: a successful dial returned no connection")
	if !c.adopt(stop, conn) {
		return nil
	}
	c.emitState(ConnState{Connected: true})
	go c.run(stop, conn)
	return nil
}

// dial opens one connection. A handshake the server rejected for its
// credentials wraps ErrUnauthorized: retrying cannot fix it.
func (c *wsClient) dial() (*websocket.Conn, error) {
	assert.That(c.url != "", "dial: empty URL")
	assert.That(c.headerName != "", "dial: empty header name")
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()

	header := http.Header{}
	header.Set(c.headerName, token)

	conn, resp, err := websocket.DefaultDialer.Dial(c.url, header)
	if err != nil {
		assert.That(conn == nil, "dial: a failed dial returned a connection")
		if resp != nil {
			errlog.Close(resp.Body, "websocket handshake response body")
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return nil, fmt.Errorf("ws: %w: %w (HTTP %d)", ErrUnauthorized, err, resp.StatusCode)
			}
			return nil, fmt.Errorf("ws: %w (HTTP %d)", err, resp.StatusCode)
		}
		return nil, err
	}
	assert.That(conn != nil, "dial: no connection without an error")
	return conn, nil
}

// adopt makes conn the live connection of the session that stop belongs
// to. If that session was closed while the dial was in flight, the
// connection is discarded and adopt reports false.
func (c *wsClient) adopt(stop chan struct{}, conn *websocket.Conn) bool {
	assert.That(stop != nil, "adopt: nil stop channel")
	assert.That(conn != nil, "adopt: nil connection")
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stop != stop {
		errlog.Debug(conn.Close(), "close websocket dialed after the session closed")
		return false
	}
	assert.That(c.conn == nil, "adopt: the session already has a live connection")
	c.conn = conn
	return true
}

// release forgets conn if it is still the live connection, and closes it.
func (c *wsClient) release(conn *websocket.Conn) {
	assert.That(conn != nil, "release: nil connection")
	c.mu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	c.mu.Unlock()
	errlog.Debug(conn.Close(), "close websocket connection")
}

// State returns the connection-state channel.
func (c *wsClient) State() <-chan ConnState {
	assert.That(c.state != nil, "State: nil state channel")
	return c.state
}

// emitState publishes a transition without ever blocking: the reader may be
// busy, and a stalled read loop would stop reconnecting altogether.
func (c *wsClient) emitState(s ConnState) {
	assert.That(c.state != nil, "emitState: nil state channel")
	assert.That(!s.Desynced || s.Connected, "emitState: a desync is reported only while connected")
	assert.That(!s.Unauthorized || !s.Connected, "emitState: an unauthorized state is a disconnect")
	select {
	case c.state <- s:
	default:
	}
}

// closed reports, without a wait, whether stop is closed.
func closed(stop chan struct{}) bool {
	// A nil channel never closes, so a loop waiting on it would never end.
	assert.That(stop != nil, "closed: nil channel")
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

// run owns one session: it reads conn until it fails, then redials, until
// stop is closed. A nil conn means the first dial failed and it starts by
// redialing. A conn given at the start is read even when stop is closed, so
// it is released (Power of 10 rule 2: the loop ends when stop is closed).
func (c *wsClient) run(stop chan struct{}, conn *websocket.Conn) {
	defer errlog.Recover("websocket session")
	assert.That(stop != nil, "run: nil stop channel")
	backoff := c.newBackoff()
	for conn != nil || !closed(stop) {
		if conn == nil {
			conn = c.redial(stop, backoff)
			if conn == nil {
				return
			}
			c.emitState(ConnState{Connected: true})
		}
		assert.That(conn != nil, "run: no connection to read")

		err := c.read(stop, conn)
		c.release(conn)
		conn = nil
		if closed(stop) {
			return
		}
		c.emitState(ConnState{Connected: false, Err: err})
	}
}

// read delivers events from conn until the connection fails or stop is
// closed. Any frame, pongs included, proves the link is alive; a link
// silent for longer than pongWait is treated as dead, since a half-open TCP
// connection would otherwise block here forever.
func (c *wsClient) read(stop chan struct{}, conn *websocket.Conn) error {
	assert.That(stop != nil, "read: nil stop channel")
	assert.That(conn != nil, "read: nil connection")
	assert.That(c.pongWait > 0, "read: pongWait must be positive")
	assert.That(c.events != nil, "read: nil events channel")
	// A deadline that cannot be set means the connection is already closed,
	// and the read below reports that.
	alive := func() {
		errlog.Debug(conn.SetReadDeadline(time.Now().Add(c.pongWait)), "set websocket read deadline")
	}
	alive()
	conn.SetPongHandler(func(string) error { alive(); return nil })
	conn.SetPingHandler(func(data string) error {
		alive()
		err := conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(writeWait))
		if errors.Is(err, websocket.ErrCloseSent) {
			return nil
		}
		return err
	})

	pinging := make(chan struct{})
	defer close(pinging)
	go c.ping(conn, pinging)

	// desynced marks an overflow already reported, so one burst of drops
	// asks for one resync rather than one per lost event.
	desynced := false
	// The loop ends when stop is closed or a read fails. The read deadline
	// makes a silent link fail (Power of 10 rule 2).
	for !closed(stop) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		alive()

		var evt model.WebSocketEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			continue
		}

		select {
		case c.events <- evt:
			desynced = false
		case <-stop:
			return nil
		default:
			// The buffer is full, so this event is lost. Dropping it quietly
			// leaves the view stale with the socket still up and nothing to
			// hint at it, so report the gap and let the reader resync.
			if !desynced {
				desynced = true
				c.emitState(ConnState{Connected: true, Desynced: true})
			}
		}
	}
	return nil
}

// ping keeps a quiet link provably alive until done is closed. WriteControl
// is safe alongside the reader and Send, so it needs no lock.
func (c *wsClient) ping(conn *websocket.Conn, done chan struct{}) {
	defer errlog.Recover("websocket ping")
	assert.That(conn != nil, "ping: nil connection")
	assert.That(done != nil, "ping: nil done channel")
	assert.That(c.pingPeriod > 0 && c.pingPeriod < c.pongWait, "ping: the ping period must be positive and shorter than pongWait")
	t := time.NewTicker(c.pingPeriod)
	defer t.Stop()
	for !closed(done) {
		select {
		case <-done:
			return
		case <-t.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}

// redial dials until it connects, stop is closed, or the credentials are
// rejected. It returns the adopted connection, or nil if the session ends.
func (c *wsClient) redial(stop chan struct{}, backoff *Backoff) *websocket.Conn {
	assert.That(stop != nil, "redial: nil stop channel")
	assert.That(backoff != nil, "redial: nil backoff")
	for !closed(stop) {
		select {
		case <-stop:
			return nil
		case <-time.After(backoff.Next()):
		}

		conn, err := c.dial()
		if err != nil {
			assert.That(conn == nil, "redial: a failed dial returned a connection")
			// Retrying cannot fix rejected credentials. Report it and stop,
			// so the UI can ask the user to sign in again instead of the
			// loop dialing forever behind a dead session.
			if errors.Is(err, ErrUnauthorized) {
				c.emitState(ConnState{Connected: false, Err: err, Unauthorized: true})
				return nil
			}
			continue
		}
		assert.That(conn != nil, "redial: a successful dial returned no connection")
		if !c.adopt(stop, conn) {
			return nil
		}
		backoff.Reset()
		return conn
	}
	return nil
}

// Close ends the session. It is safe to call more than once, and Connect
// may start a new session afterwards.
func (c *wsClient) Close() error {
	c.mu.Lock()
	stop, conn := c.stop, c.conn
	// A connection is adopted only by a running session.
	assert.That(conn == nil || stop != nil, "Close: a connection without a session")
	c.stop, c.conn = nil, nil
	c.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if conn != nil {
		// Closing the connection unblocks the reader, which then sees stop.
		return conn.Close()
	}
	return nil
}

// SetToken updates the token used for reconnections.
func (c *wsClient) SetToken(token string) {
	assert.That(c != nil, "SetToken: nil client")
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

func (c *wsClient) Events() <-chan model.WebSocketEvent {
	assert.That(c.events != nil, "Events: nil events channel")
	return c.events
}
