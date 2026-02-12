package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsClient wraps a WebSocket connection with a write mutex to prevent
// concurrent writes, which gorilla/websocket does not support.
type wsClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient) writeMessage(msgType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(msgType, data)
}

// Hub manages WebSocket clients and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}

	streamMu     sync.RWMutex
	activeStreams map[string]map[string]any // streamId -> log-stream-start payload
}

// New creates a new WebSocket Hub.
func New() *Hub {
	return &Hub{
		clients:      make(map[*wsClient]struct{}),
		activeStreams: make(map[string]map[string]any),
	}
}

// HandleWS is a Gin handler that upgrades HTTP to WebSocket and registers the client.
func (h *Hub) HandleWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	client := &wsClient{conn: conn}

	// Send current active streams snapshot to the new client
	h.streamMu.RLock()
	for _, payload := range h.activeStreams {
		msg := map[string]any{"event": "log-stream-start", "data": payload}
		if data, err := json.Marshal(msg); err == nil {
			client.writeMessage(websocket.TextMessage, data)
		}
	}
	h.streamMu.RUnlock()

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	// Read loop — keep connection alive and detect disconnects.
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, client)
			h.mu.Unlock()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// TrackStream registers an active stream. Called when log-stream-start is broadcast.
func (h *Hub) TrackStream(streamId string, data map[string]any) {
	h.streamMu.Lock()
	h.activeStreams[streamId] = data
	h.streamMu.Unlock()
}

// UntrackStream removes an active stream. Called on log-stream-end or log-created.
func (h *Hub) UntrackStream(streamId string) {
	h.streamMu.Lock()
	delete(h.activeStreams, streamId)
	h.streamMu.Unlock()
}

// Broadcast sends a JSON event to all connected WebSocket clients.
// Matches the BroadcastFunc signature: func(event string, data ...any)
func (h *Hub) Broadcast(event string, data ...any) {
	msg := map[string]any{"event": event}
	if len(data) > 0 {
		msg["data"] = data[0]
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ws] marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if err := client.writeMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("[ws] write error: %v", err)
			// Close will be handled by the read loop detecting the broken connection
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
