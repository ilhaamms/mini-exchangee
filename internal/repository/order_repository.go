package repository

import (
	"fmt"
	"sort"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

// OrderRepository provides thread-safe in-memory storage for orders
type OrderRepository struct {
	mu     sync.RWMutex
	orders map[string]*domain.Order   // orderID -> Order
	byStock map[string][]*domain.Order // stockCode -> orders (FIFO)
}

// NewOrderRepository creates a new OrderRepository
func NewOrderRepository() *OrderRepository {
	return &OrderRepository{
		orders:  make(map[string]*domain.Order),
		byStock: make(map[string][]*domain.Order),
	}
}

// Save persists an order to the repository
func (r *OrderRepository) Save(order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.orders[order.ID] = order
	r.byStock[order.StockCode] = append(r.byStock[order.StockCode], order)
	return nil
}

// GetByID retrieves an order by its ID
func (r *OrderRepository) GetByID(id string) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, ok := r.orders[id]
	if !ok {
		return nil, fmt.Errorf("order %s not found", id)
	}
	return order, nil
}

// GetAll returns all orders, optionally filtered by stock and status
func (r *OrderRepository) GetAll(stockCode string, status string) []domain.Order {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []domain.Order

	for _, order := range r.orders {
		if stockCode != "" && order.StockCode != stockCode {
			continue
		}
		if status != "" && string(order.GetStatus()) != status {
			continue
		}
		results = append(results, order.ToSnapshot())
	}

	// Sort by creation time descending (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results
}

// GetOpenOrdersByStock returns all open/partial orders for a stock, sorted by FIFO
func (r *OrderRepository) GetOpenOrdersByStock(stockCode string, side domain.Side) []*domain.Order {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*domain.Order
	for _, order := range r.byStock[stockCode] {
		s := order.GetStatus()
		if order.Side == side && (s == domain.OrderStatusOpen || s == domain.OrderStatusPartial) {
			results = append(results, order)
		}
	}

	// FIFO: sort by creation time ascending
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})

	return results
}
