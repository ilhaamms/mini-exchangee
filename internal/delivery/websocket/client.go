package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 4096

	// Buffered channel size per client for outgoing messages.
	// If the buffer is full, messages are dropped (non-blocking broadcast).
	sendBufferSize = 256
)

// Client represents a WebSocket client connection
type Client struct {
	hub           *Hub
	conn          *ws.Conn
	send          chan []byte
	subscriptions map[string]bool // channel keys like "market.ticker:BBCA"
	mu            sync.RWMutex
	id            string
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *ws.Conn, id string) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan []byte, sendBufferSize),
		subscriptions: make(map[string]bool),
		id:            id,
	}
}

// Subscribe adds a subscription for a channel+stock combination
func (c *Client) Subscribe(channel, stockCode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := channel + ":" + stockCode
	c.subscriptions[key] = true
	log.Printf("WS: client %s subscribed to %s", c.id, key)
}

// Unsubscribe removes a subscription
func (c *Client) Unsubscribe(channel, stockCode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := channel + ":" + stockCode
	delete(c.subscriptions, key)
	log.Printf("WS: client %s unsubscribed from %s", c.id, key)
}

// IsSubscribed checks if client is subscribed to a channel+stock
func (c *Client) IsSubscribed(channel, stockCode string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := channel + ":" + stockCode
	return c.subscriptions[key]
}

// ReadPump reads messages from the WebSocket connection.
// It handles subscribe/unsubscribe commands from the client.
//
// Expected message format:
//
//	{"action": "subscribe", "channel": "market.ticker", "stock_code": "BBCA"}
//	{"action": "unsubscribe", "channel": "market.trade", "stock_code": "BBCA"}
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				log.Printf("WS: read error from client %s: %v", c.id, err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection.
// A goroutine per client ensures writes don't block each other.
//
// Goroutine Leak Prevention:
// - WritePump exits when the send channel is closed (by Hub.unregister)
// - The ticker is stopped on exit to prevent timer goroutine leaks
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(ws.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(ws.TextMessage, message); err != nil {
				log.Printf("WS: write error to client %s: %v", c.id, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(ws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WSMessage represents an incoming WebSocket message from a client
type WSMessage struct {
	Action    string `json:"action"`
	Channel   string `json:"channel"`
	StockCode string `json:"stock_code"`
}

// WSResponse represents an outgoing WebSocket message to a client
type WSResponse struct {
	Type      string      `json:"type"`
	Channel   string      `json:"channel"`
	StockCode string      `json:"stock_code,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message,omitempty"`
}

func (c *Client) handleMessage(message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		c.sendResponse(WSResponse{
			Type:    "error",
			Message: "invalid message format, expected JSON with action/channel/stock_code",
		})
		return
	}

	switch msg.Action {
	case "subscribe":
		if msg.Channel == "" || msg.StockCode == "" {
			c.sendResponse(WSResponse{
				Type:    "error",
				Message: "channel and stock_code are required for subscribe",
			})
			return
		}
		c.Subscribe(msg.Channel, msg.StockCode)
		c.sendResponse(WSResponse{
			Type:      "subscribed",
			Channel:   msg.Channel,
			StockCode: msg.StockCode,
			Message:   "successfully subscribed",
		})

	case "unsubscribe":
		if msg.Channel == "" || msg.StockCode == "" {
			c.sendResponse(WSResponse{
				Type:    "error",
				Message: "channel and stock_code are required for unsubscribe",
			})
			return
		}
		c.Unsubscribe(msg.Channel, msg.StockCode)
		c.sendResponse(WSResponse{
			Type:      "unsubscribed",
			Channel:   msg.Channel,
			StockCode: msg.StockCode,
			Message:   "successfully unsubscribed",
		})

	default:
		c.sendResponse(WSResponse{
			Type:    "error",
			Message: "unknown action: " + msg.Action + ". Use 'subscribe' or 'unsubscribe'",
		})
	}
}

func (c *Client) sendResponse(resp WSResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	// Non-blocking send
	select {
	case c.send <- data:
	default:
		log.Printf("WS: send buffer full for client %s, dropping message", c.id)
	}
}
