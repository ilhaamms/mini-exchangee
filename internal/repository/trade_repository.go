package repository

import (
	"sort"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

// TradeRepository provides thread-safe in-memory storage for trades
type TradeRepository struct {
	mu      sync.RWMutex
	trades  []*domain.Trade
	byStock map[string][]*domain.Trade
}

// NewTradeRepository creates a new TradeRepository
func NewTradeRepository() *TradeRepository {
	return &TradeRepository{
		trades:  make([]*domain.Trade, 0),
		byStock: make(map[string][]*domain.Trade),
	}
}

// Save persists a trade to the repository
func (r *TradeRepository) Save(trade *domain.Trade) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.trades = append(r.trades, trade)
	r.byStock[trade.StockCode] = append(r.byStock[trade.StockCode], trade)
	return nil
}

// GetAll returns all trades sorted by time descending
func (r *TradeRepository) GetAll() []domain.Trade {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]domain.Trade, len(r.trades))
	for i, t := range r.trades {
		results[i] = *t
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results
}

// GetByStock returns trades for a specific stock code
func (r *TradeRepository) GetByStock(stockCode string) []domain.Trade {
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

	return results
}

// GetRecentByStock returns the last N trades for a stock
func (r *TradeRepository) GetRecentByStock(stockCode string, limit int) []domain.Trade {
	trades := r.GetByStock(stockCode)
	if len(trades) > limit {
		return trades[:limit]
	}
	return trades
}
