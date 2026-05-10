package repository

import (
	"sort"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

type InMemoryTradeRepository struct {
	mu      sync.RWMutex
	trades  []*domain.Trade
	byStock map[string][]*domain.Trade
}

func NewTradeRepository() *InMemoryTradeRepository {
	return &InMemoryTradeRepository{
		trades:  make([]*domain.Trade, 0),
		byStock: make(map[string][]*domain.Trade),
	}
}

func (r *InMemoryTradeRepository) Save(trade *domain.Trade) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.trades = append(r.trades, trade)
	r.byStock[trade.StockCode] = append(r.byStock[trade.StockCode], trade)
	return nil
}

func (r *InMemoryTradeRepository) GetAll() ([]domain.Trade, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]domain.Trade, len(r.trades))
	for i, t := range r.trades {
		results[i] = *t
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}

func (r *InMemoryTradeRepository) GetByStock(stockCode string) ([]domain.Trade, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stockTrades := r.byStock[stockCode]
	results := make([]domain.Trade, len(stockTrades))
	for i, t := range stockTrades {
		results[i] = *t
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}

func (r *InMemoryTradeRepository) GetRecentByStock(stockCode string, limit int) ([]domain.Trade, error) {
	trades, err := r.GetByStock(stockCode)
	if err != nil {
		return nil, err
	}
	if len(trades) > limit {
		return trades[:limit], nil
	}
	return trades, nil
}
