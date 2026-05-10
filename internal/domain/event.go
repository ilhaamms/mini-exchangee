package domain

type EventType string

const (
	EventTicker      EventType = "market.ticker"
	EventTrade       EventType = "market.trade"
	EventOrderBook   EventType = "market.orderbook"
	EventOrderUpdate EventType = "order.update"
)

type Event struct {
	Type      EventType   `json:"type"`
	Channel   string      `json:"channel"`
	StockCode string      `json:"stock_code"`
	Data      interface{} `json:"data"`
}
