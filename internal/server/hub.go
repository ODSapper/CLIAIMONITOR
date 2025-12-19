package server

import (
	"encoding/json"
	"sync"

	"github.com/CLIAIMONITOR/internal/types"
	"github.com/gorilla/websocket"
)

// WebSocket buffer and channel size constants
const (
	// WebSocketBufferSize is the buffer size for WebSocket send/broadcast channels
	// Allows pending messages to queue up before blocking, useful for burst traffic
	WebSocketBufferSize = 256
)

// Client represents a WebSocket client (browser)
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub manages WebSocket clients
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, WebSocketBufferSize),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Register adds a client
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// BroadcastJSON sends a JSON message to all clients
func (h *Hub) BroadcastJSON(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

// BroadcastState sends full state to all clients
func (h *Hub) BroadcastState(state *types.DashboardState) {
	h.BroadcastJSON(types.WSMessage{
		Type: types.WSTypeStateUpdate,
		Data: state,
	})
}

// BroadcastAlert sends an alert to all clients
func (h *Hub) BroadcastAlert(alert *types.Alert) {
	h.BroadcastJSON(types.WSMessage{
		Type: types.WSTypeAlert,
		Data: alert,
	})
}

// BroadcastActivity sends an activity log entry
func (h *Hub) BroadcastActivity(activity *types.ActivityLog) {
	h.BroadcastJSON(types.WSMessage{
		Type: types.WSTypeActivity,
		Data: activity,
	})
}

// BroadcastEscalation sends an escalation message to all clients
func (h *Hub) BroadcastEscalation(escalation interface{}) {
	h.BroadcastJSON(types.WSMessage{
		Type: types.WSTypeEscalation,
		Data: escalation,
	})
}

// BroadcastCaptainMessage sends a Captain response to the dashboard chat
func (h *Hub) BroadcastCaptainMessage(text string) {
	h.BroadcastJSON(types.WSMessage{
		Type: types.WSTypeCaptainMessage,
		Data: map[string]string{"text": text},
	})
}

// BroadcastChat sends a chat message to all WebSocket clients
func (h *Hub) BroadcastChat(msg *types.ChatMessage) {
	h.BroadcastJSON(types.WSMessage{
		Type: types.WSTypeChat,
		Data: msg,
	})
}

// ClientCount returns number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// readPump reads messages from the WebSocket
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// We don't process incoming messages from browser currently
	}
}

// writePump writes messages to the WebSocket
func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}
