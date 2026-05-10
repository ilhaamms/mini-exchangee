package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan domain.Event
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan domain.Event, 1024), 
	}
}

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
				return 
			}
			h.broadcastEvent(event)
		}
	}
}

func (h *Hub) BroadcastEvent(event domain.Event) {
	select {
	case h.broadcast <- event:
	default:
		log.Printf("HUB: broadcast channel full, dropping event for %s:%s", event.Channel, event.StockCode)
	}
}

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
			
			select {
			case client.send <- message:
			default:
				log.Printf("HUB: client %s too slow, dropping message for %s:%s",
					client.id, event.Channel, event.StockCode)
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
