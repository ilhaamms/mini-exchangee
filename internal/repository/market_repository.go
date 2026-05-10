package repository

import (
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

type MarketRepository struct {
	mu      sync.RWMutex
	markets map[string]*domain.MarketData 
}

func NewMarketRepository() *MarketRepository {
	return &MarketRepository{
		markets: make(map[string]*domain.MarketData),
	}
}

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

func (r *MarketRepository) Get(stockCode string) *domain.MarketData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.markets[stockCode]
}

func (r *MarketRepository) GetAll() map[string]domain.Ticker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]domain.Ticker)
	for code, md := range r.markets {
		result[code] = md.GetTicker()
	}
	return result
}

func (r *MarketRepository) GetAllStockCodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	codes := make([]string, 0, len(r.markets))
	for code := range r.markets {
		codes = append(codes, code)
	}
	return codes
}
