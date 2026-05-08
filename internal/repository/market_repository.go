package repository

import (
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

// MarketRepository provides thread-safe in-memory storage for market data
type MarketRepository struct {
	mu      sync.RWMutex
	markets map[string]*domain.MarketData // stockCode -> MarketData
}

// NewMarketRepository creates a new MarketRepository
func NewMarketRepository() *MarketRepository {
	return &MarketRepository{
		markets: make(map[string]*domain.MarketData),
	}
}

// GetOrCreate returns existing market data or creates new one with initial price
func (r *MarketRepository) GetOrCreate(stockCode string, initialPrice float64) *domain.MarketData {
	r.mu.Lock()
	defer r.mu.Unlock()

	if md, ok := r.markets[stockCode]; ok {
		return md
	}

	md := domain.NewMarketData(stockCode, initialPrice)
	r.markets[stockCode] = md
	return md
}

// Get returns market data for a specific stock
func (r *MarketRepository) Get(stockCode string) *domain.MarketData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.markets[stockCode]
}

// GetAll returns all market data
func (r *MarketRepository) GetAll() map[string]domain.Ticker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]domain.Ticker)
	for code, md := range r.markets {
		result[code] = md.GetTicker()
	}
	return result
}

// GetAllStockCodes returns all registered stock codes
func (r *MarketRepository) GetAllStockCodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	codes := make([]string, 0, len(r.markets))
	for code := range r.markets {
		codes = append(codes, code)
	}
	return codes
}
