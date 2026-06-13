package api

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"

	"EveTrace/internal/metrics"
	"EveTrace/pkg/core"
)

// Client is a single WebSocket connection managed by the Hub.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub maintains the set of active WebSocket clients and broadcasts live events
// to all of them. Only events with Live=true are forwarded; historical replay
// events are suppressed so the frontend never receives stale data on reconnect.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool

	// Broadcast accepts JSON-encoded messages to send to all connected clients.
	broadcast chan []byte

	register   chan *Client
	unregister chan *Client
}

// NewHub creates an idle Hub. Call Run to start processing.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
	}
}

// Run processes register/unregister/broadcast events until the done channel
// is closed. Call in a dedicated goroutine.
func (h *Hub) Run(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
			metrics.WSClients.Add(1)
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
				metrics.WSClients.Add(-1)
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Slow client — drop the message rather than blocking.
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Send encodes a live core.Event as JSON and queues it for broadcast.
// Non-live (replay) events are silently dropped.
func (h *Hub) Send(ev core.Event) {
	if !ev.Live {
		return
	}
	h.broadcastJSON(ev)
}

// SendDiagnostic encodes a core.LogEvent as JSON and queues it for broadcast.
func (h *Hub) SendDiagnostic(ev core.LogEvent) {
	h.broadcastJSON(ev)
}

func (h *Hub) broadcastJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case h.broadcast <- b:
	default:
	}
}

// writePump forwards messages from Client.send to the WebSocket connection.
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// readPump drains incoming frames so the connection stays healthy. We don't
// process client messages, but we must read to handle pings and detect closes.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
