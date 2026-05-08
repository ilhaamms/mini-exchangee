package domain

import "time"

// Trade represents a completed trade between a buy and sell order
type Trade struct {
	ID           string  `json:"id"`
	StockCode    string  `json:"stock_code"`
	BuyOrderID   string  `json:"buy_order_id"`
	SellOrderID  string  `json:"sell_order_id"`
	Price        float64 `json:"price"`
	Quantity     int64   `json:"quantity"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewTrade creates a new trade record
func NewTrade(id, stockCode, buyOrderID, sellOrderID string, price float64, quantity int64) *Trade {
	return &Trade{
		ID:          id,
		StockCode:   stockCode,
		BuyOrderID:  buyOrderID,
		SellOrderID: sellOrderID,
		Price:       price,
		Quantity:    quantity,
		CreatedAt:   time.Now(),
	}
}
