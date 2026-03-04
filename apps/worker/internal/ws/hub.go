package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
)

const (
	// Per-client outbound message buffer size.
	clientSendBuf = 64
	// Write deadline for WebSocket writes.
	writeWait = 5 * time.Second
)

// wsClient wraps a WebSocket connection with a buffered send channel.
// A dedicated goroutine drains the channel and writes to the connection.
type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// writePump drains the send channel and writes messages to the WebSocket.
// It exits when the send channel is closed, closing the connection on the way out.
func (c *wsClient) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// Hub manages WebSocket clients and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}

	streamMu      sync.RWMutex
	activeStreams map[string]map[string]any // streamId -> log-stream-start payload

	jwtSecret string
}

// New creates a new WebSocket Hub.
func New(jwtSecret string) *Hub {
	return &Hub{
		clients:       make(map[*wsClient]struct{}),
		activeStreams: make(map[string]map[string]any),
		jwtSecret:     jwtSecret,
	}
}

// HandleWS is a Gin handler that upgrades HTTP to WebSocket and registers the client.
func (h *Hub) HandleWS(c *gin.Context) {
	// Verify JWT token from query parameter
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	// Strip token from the URL so Gin's Logger middleware won't record it.
	q := c.Request.URL.Query()
	q.Del("token")
	c.Request.URL.RawQuery = q.Encode()

	if _, err := middleware.VerifyJWT(token, h.jwtSecret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	u := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := u.Upgrade(c.Writer, c.Request, http.Header{})
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, clientSendBuf),
	}

	// Send current active streams snapshot to the new client
	h.streamMu.RLock()
	for _, payload := range h.activeStreams {
		msg := map[string]any{"event": "log-stream-start", "data": payload}
		if data, err := json.Marshal(msg); err == nil {
			select {
			case client.send <- data:
			default:
			}
		}
	}
	h.streamMu.RUnlock()

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	// Start write pump
	go client.writePump()

	// Read loop — keep connection alive and detect disconnects.
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, client)
			h.mu.Unlock()
			close(client.send) // stops writePump → closes conn
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
// Non-blocking: messages are dropped for clients whose buffer is full.
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
		select {
		case client.send <- payload:
		default:
			// Client buffer full — drop message to avoid blocking the caller.
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
