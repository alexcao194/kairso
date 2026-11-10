// Package realtime broadcasts data-change events over WebSocket to every connected
// client that belongs to the affected workspace, so other members and the same
// user's other devices/tabs can refresh immediately instead of waiting for FCM
// or a manual pull-to-refresh.
package realtime

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Event is the payload broadcast to every WebSocket connection subscribed to
// WorkspaceID. For "*.created"/"*.updated" (and similar mutation) events,
// Entity carries the full up-to-date resource (the same shape as its REST
// response) so clients can patch their local state directly instead of
// refetching. "*.deleted" events carry no Entity; clients drop EntityID from
// their local state instead.
type Event struct {
	Type        string `json:"type"`
	WorkspaceID string `json:"workspaceId"`
	EntityID    string `json:"entityId,omitempty"`
	ActorID     string `json:"actorId,omitempty"`
	// ActorConnID identifies the specific client connection (browser tab /
	// app instance) that caused this event, distinct from ActorID (the
	// user) — set from the X-Conn-Id header on the mutating REST request.
	// Broadcast uses it to skip re-delivering the event to the exact
	// connection that already has this change applied locally from its own
	// REST response, while still delivering to the same user's *other*
	// open tabs/devices (those have a different connID). Never sent to
	// clients (json:"-"): it's dispatch-only plumbing, not part of the
	// wire payload they consume.
	ActorConnID string `json:"-"`
	Entity      any    `json:"entity,omitempty"`
}

type Broadcaster interface {
	Broadcast(event Event)
}

var upgrader = websocket.Upgrader{
	// Native apps and the web client are served from different origins
	// (app.kairso.com, api.kairso.com); the WS handshake is authenticated via
	// JWT instead of relying on same-origin checks.
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingInterval   = (pongWait * 9) / 10
	sendBufferSize = 32
)

type conn struct {
	ws     *websocket.Conn
	send   chan []byte
	rooms  []string
	connID string
}

// Hub keeps track of live WebSocket connections grouped by workspace room.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*conn]struct{}
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[*conn]struct{})}
}

// Broadcast fans an event out to every connection currently subscribed to
// event.WorkspaceID, including the actor's own other devices/tabs — except
// the exact connection identified by event.ActorConnID (if set), which
// already has this change applied locally from its own REST response.
func (h *Hub) Broadcast(event Event) {
	if event.WorkspaceID == "" {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	targets := make([]*conn, 0, len(h.rooms[event.WorkspaceID]))
	for c := range h.rooms[event.WorkspaceID] {
		if event.ActorConnID != "" && c.connID == event.ActorConnID {
			continue
		}
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		select {
		case c.send <- payload:
		default:
			// Slow consumer; drop rather than block the broadcaster.
		}
	}
}

// ServeWS upgrades the request to a WebSocket connection subscribed to the
// given workspace rooms (the caller resolves these from the authenticated
// user). connID is the client-generated id sent as the `connId` query
// param — empty is valid (an older client, or one that opted out), it just
// means this connection is never skipped as "the actor's own".
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, workspaceIDs []string, connID string) error {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	c := &conn{ws: ws, send: make(chan []byte, sendBufferSize), rooms: workspaceIDs, connID: connID}
	h.register(c)
	go c.writePump()
	c.readPump(h)
	return nil
}

func (h *Hub) register(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, room := range c.rooms {
		if h.rooms[room] == nil {
			h.rooms[room] = make(map[*conn]struct{})
		}
		h.rooms[room][c] = struct{}{}
	}
}

func (h *Hub) unregister(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, room := range c.rooms {
		delete(h.rooms[room], c)
		if len(h.rooms[room]) == 0 {
			delete(h.rooms, room)
		}
	}
}

func (c *conn) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *conn) readPump(h *Hub) {
	defer func() {
		h.unregister(c)
		close(c.send)
		c.ws.Close()
	}()
	c.ws.SetReadLimit(512)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		// Clients don't send app messages, only pong frames (handled above)
		// and close frames; this loop just blocks until the connection drops.
		if _, _, err := c.ws.ReadMessage(); err != nil {
			return
		}
	}
}
