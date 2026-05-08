package domain

// EventType represents the type of event emitted by the system
type EventType string

const (
	EventTicker      EventType = "market.ticker"
	EventTrade       EventType = "market.trade"
	EventOrderBook   EventType = "market.orderbook"
	EventOrderUpdate EventType = "order.update"
)

// Event represents a system event that gets broadcast via WebSocket
type Event struct {
	Type      EventType   `json:"type"`
	Channel   string      `json:"channel"`
	StockCode string      `json:"stock_code"`
	Data      interface{} `json:"data"`
}
