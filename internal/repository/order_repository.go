package repository

import (
	"fmt"
	"sort"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

type InMemoryOrderRepository struct {
	mu      sync.RWMutex
	orders  map[string]*domain.Order   
	byStock map[string][]*domain.Order 
}

func NewOrderRepository() *InMemoryOrderRepository {
	return &InMemoryOrderRepository{
		orders:  make(map[string]*domain.Order),
		byStock: make(map[string][]*domain.Order),
	}
}

func (r *InMemoryOrderRepository) Save(order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.orders[order.ID] = order
	r.byStock[order.StockCode] = append(r.byStock[order.StockCode], order)
	return nil
}

func (r *InMemoryOrderRepository) GetByID(id string) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, ok := r.orders[id]
	if !ok {
		return nil, fmt.Errorf("order %s not found", id)
	}
	return order, nil
}

func (r *InMemoryOrderRepository) GetAll(stockCode string, status string) ([]domain.Order, error) {
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

	
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}

func (r *InMemoryOrderRepository) UpdateStatus(order *domain.Order) error {
	return nil
}

func (r *InMemoryOrderRepository) GetOpenOrdersByStock(stockCode string, side domain.Side) ([]*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*domain.Order
	for _, order := range r.byStock[stockCode] {
		s := order.GetStatus()
		if order.Side == side && (s == domain.OrderStatusOpen || s == domain.OrderStatusPartial) {
			results = append(results, order)
		}
	}

	
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})

	return results, nil
}
