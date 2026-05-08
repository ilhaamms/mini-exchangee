package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

// Hub maintains the set of active clients and broadcasts events to them.
//
// Non-Blocking Broadcast Strategy:
// - Each client has a buffered send channel (256 messages)
// - Broadcast uses select with default to skip slow clients
// - Slow clients that can't keep up will have messages dropped
// - This ensures one slow client doesn't block other clients
//
// Goroutine Leak Prevention:
// - When a client disconnects, it sends itself to unregister channel
// - Hub removes it from clients map and closes the send channel
// - Closing send channel causes WritePump to exit
// - ReadPump exits on read error and triggers unregister
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan domain.Event
	mu         sync.RWMutex
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan domain.Event, 1024), // buffered to prevent blocking producers
	}
}

// Run starts the hub's event loop. Must be run as a goroutine.
// The hub exits only when the broadcast channel is closed (application shutdown).
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("HUB: client %s connected (total: %d)", client.id, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("HUB: client %s disconnected (total: %d)", client.id, len(h.clients))
			}
			h.mu.Unlock()

		case event, ok := <-h.broadcast:
			if !ok {
				return // channel closed, shutdown
			}
			h.broadcastEvent(event)
		}
	}
}

// BroadcastEvent sends an event to the broadcast channel (non-blocking)
func (h *Hub) BroadcastEvent(event domain.Event) {
	select {
	case h.broadcast <- event:
	default:
		log.Printf("HUB: broadcast channel full, dropping event for %s:%s", event.Channel, event.StockCode)
	}
}

// broadcastEvent sends an event to all subscribed clients (non-blocking per client)
func (h *Hub) broadcastEvent(event domain.Event) {
	message, err := json.Marshal(WSResponse{
		Type:      string(event.Type),
		Channel:   event.Channel,
		StockCode: event.StockCode,
		Data:      event.Data,
	})
	if err != nil {
		log.Printf("HUB: failed to marshal event: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.IsSubscribed(event.Channel, event.StockCode) {
			// Non-blocking send: if client buffer is full, skip it
			select {
			case client.send <- message:
			default:
				log.Printf("HUB: client %s too slow, dropping message for %s:%s",
					client.id, event.Channel, event.StockCode)
			}
		}
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
