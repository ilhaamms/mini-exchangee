package domain

import (
	"sync"
	"time"
)

// Side represents the order side (BUY or SELL)
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// OrderStatus represents the current status of an order
type OrderStatus string

const (
	OrderStatusOpen      OrderStatus = "OPEN"
	OrderStatusFilled    OrderStatus = "FILLED"
	OrderStatusPartial   OrderStatus = "PARTIAL"
	OrderStatusCancelled OrderStatus = "CANCELLED"
)

// Order represents a trading order in the system
type Order struct {
	mu            sync.RWMutex `json:"-"`
	ID            string       `json:"id"`
	StockCode     string       `json:"stock_code"`
	Side          Side         `json:"side"`
	Price         float64      `json:"price"`
	Quantity      int64        `json:"quantity"`
	FilledQty     int64        `json:"filled_qty"`
	RemainingQty  int64        `json:"remaining_qty"`
	Status        OrderStatus  `json:"status"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// NewOrder creates a new order with the given parameters
func NewOrder(id, stockCode string, side Side, price float64, quantity int64) *Order {
	now := time.Now()
	return &Order{
		ID:           id,
		StockCode:    stockCode,
		Side:         side,
		Price:        price,
		Quantity:     quantity,
		FilledQty:    0,
		RemainingQty: quantity,
		Status:       OrderStatusOpen,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Fill reduces remaining quantity and updates status (thread-safe)
func (o *Order) Fill(qty int64) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.FilledQty += qty
	o.RemainingQty -= qty
	o.UpdatedAt = time.Now()

	if o.RemainingQty <= 0 {
		o.RemainingQty = 0
		o.Status = OrderStatusFilled
	} else {
		o.Status = OrderStatusPartial
	}
}

// GetRemainingQty returns remaining quantity (thread-safe)
func (o *Order) GetRemainingQty() int64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.RemainingQty
}

// GetStatus returns current status (thread-safe)
func (o *Order) GetStatus() OrderStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.Status
}

// ToSnapshot returns a copy of the order for safe reading
func (o *Order) ToSnapshot() Order {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return Order{
		ID:           o.ID,
		StockCode:    o.StockCode,
		Side:         o.Side,
		Price:        o.Price,
		Quantity:     o.Quantity,
		FilledQty:    o.FilledQty,
		RemainingQty: o.RemainingQty,
		Status:       o.Status,
		CreatedAt:    o.CreatedAt,
		UpdatedAt:    o.UpdatedAt,
	}
}
