package api

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub manages all active WebSocket connections and fans out broadcasts.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	bcast   chan []byte
	logger  *log.Logger
}

func NewHub(logger *log.Logger) *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		bcast:   make(chan []byte, 256),
		logger:  logger,
	}
}

// Broadcast enqueues a message for delivery to all connected clients.
func (h *Hub) Broadcast(payload []byte) {
	select {
	case h.bcast <- payload:
	default:
		h.logger.Println("ws: broadcast channel full, dropping message")
	}
}

// Run is the hub's event loop — must be started as a goroutine.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-h.bcast:
			h.fanOut(msg)
		case <-ticker.C:
			h.fanOut([]byte(`{"type":"ping"}`))
		}
	}
}

func (h *Hub) fanOut(msg []byte) {
	h.mu.RLock()
	snapshot := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		snapshot = append(snapshot, c)
	}
	h.mu.RUnlock()

	for _, c := range snapshot {
		select {
		case c.send <- msg:
		default:
			// Slow consumer: evict
			h.evict(c)
		}
	}
}

func (h *Hub) evict(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// ServeWS upgrades an HTTP connection to WebSocket and registers it with the hub.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("ws: upgrade error: %v", err)
		return
	}

	c := &client{conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump(h)
	go c.readPump(h)
}

func (c *client) writePump(h *Hub) {
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *client) readPump(h *Hub) {
	defer func() {
		h.evict(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
