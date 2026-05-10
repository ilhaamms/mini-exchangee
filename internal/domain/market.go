package domain

import (
	"sync"
	"time"
)

type Ticker struct {
	StockCode  string  `json:"stock_code"`
	LastPrice  float64 `json:"last_price"`
	PrevPrice  float64 `json:"prev_price"`
	Change     float64 `json:"change"`
	ChangePct  float64 `json:"change_pct"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	Volume     int64   `json:"volume"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type OrderBookEntry struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	Count    int     `json:"count"`
}

type OrderBook struct {
	StockCode string           `json:"stock_code"`
	Bids      []OrderBookEntry `json:"bids"`
	Asks      []OrderBookEntry `json:"asks"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type MarketData struct {
	mu        sync.RWMutex
	Ticker    Ticker
	OpenPrice float64
}

func NewMarketData(stockCode string, initialPrice float64) *MarketData {
	now := time.Now()
	return &MarketData{
		Ticker: Ticker{
			StockCode: stockCode,
			LastPrice: initialPrice,
			PrevPrice: initialPrice,
			Change:    0,
			ChangePct: 0,
			High:      initialPrice,
			Low:       initialPrice,
			Volume:    0,
			UpdatedAt: now,
		},
		OpenPrice: initialPrice,
	}
}

func (m *MarketData) UpdatePrice(price float64, volume int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Ticker.PrevPrice = m.Ticker.LastPrice
	m.Ticker.LastPrice = price
	m.Ticker.Change = price - m.OpenPrice
	if m.OpenPrice > 0 {
		m.Ticker.ChangePct = (m.Ticker.Change / m.OpenPrice) * 100
	}
	if price > m.Ticker.High {
		m.Ticker.High = price
	}
	if price < m.Ticker.Low {
		m.Ticker.Low = price
	}
	m.Ticker.Volume += volume
	m.Ticker.UpdatedAt = time.Now()
}

func (m *MarketData) GetTicker() Ticker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Ticker
}
