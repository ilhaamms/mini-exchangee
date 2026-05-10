package domain

import (
	"sync"
	"time"
)

type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

type OrderStatus string

const (
	OrderStatusOpen      OrderStatus = "OPEN"
	OrderStatusFilled    OrderStatus = "FILLED"
	OrderStatusPartial   OrderStatus = "PARTIAL"
	OrderStatusCancelled OrderStatus = "CANCELLED"
)

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

func (o *Order) GetRemainingQty() int64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.RemainingQty
}

func (o *Order) GetStatus() OrderStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.Status
}

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
