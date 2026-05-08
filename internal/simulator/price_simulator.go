package simulator

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/repository"
)

// EventCallback is called when price changes
type EventCallback func(event domain.Event)

// StockConfig defines configuration for a simulated stock
type StockConfig struct {
	Code         string
	InitialPrice float64
	Volatility   float64 // percentage of price change per tick (e.g., 0.005 = 0.5%)
}

// PriceSimulator simulates periodic price changes for configured stocks.
//
// How the simulation works:
// 1. Each stock has a goroutine that generates price ticks at random intervals
// 2. Price changes follow a random walk model: new_price = old_price * (1 + random(-volatility, +volatility))
// 3. Price is bounded to prevent going below 0.01
// 4. Each tick emits a market.ticker event to the WebSocket hub
// 5. Tick interval is randomized between 500ms-3000ms to simulate natural market behavior
//
// Goroutine Lifecycle:
// - Each stock simulation runs in its own goroutine
// - Goroutines are stopped when Stop() is called via the done channel
// - WaitGroup ensures clean shutdown
type PriceSimulator struct {
	marketRepo *repository.MarketRepository
	onEvent    EventCallback
	configs    []StockConfig
	done       chan struct{}
	wg         sync.WaitGroup
}

// DefaultStocks returns a default set of Indonesian stocks for simulation
func DefaultStocks() []StockConfig {
	return []StockConfig{
		{Code: "BBCA", InitialPrice: 9500.0, Volatility: 0.003},
		{Code: "BBRI", InitialPrice: 5200.0, Volatility: 0.004},
		{Code: "TLKM", InitialPrice: 3800.0, Volatility: 0.005},
		{Code: "ASII", InitialPrice: 6100.0, Volatility: 0.003},
		{Code: "BMRI", InitialPrice: 6800.0, Volatility: 0.004},
	}
}

// NewPriceSimulator creates a new PriceSimulator
func NewPriceSimulator(
	marketRepo *repository.MarketRepository,
	onEvent EventCallback,
	configs []StockConfig,
) *PriceSimulator {
	return &PriceSimulator{
		marketRepo: marketRepo,
		onEvent:    onEvent,
		configs:    configs,
		done:       make(chan struct{}),
	}
}

// Start begins the price simulation for all configured stocks
func (s *PriceSimulator) Start() {
	log.Printf("SIMULATOR: starting price simulation for %d stocks", len(s.configs))

	// Initialize market data for all stocks
	for _, cfg := range s.configs {
		s.marketRepo.GetOrCreate(cfg.Code, cfg.InitialPrice)
	}

	// Start a goroutine per stock
	for _, cfg := range s.configs {
		s.wg.Add(1)
		go s.simulateStock(cfg)
	}
}

// Stop gracefully stops all simulation goroutines
func (s *PriceSimulator) Stop() {
	log.Println("SIMULATOR: stopping price simulation")
	close(s.done)
	s.wg.Wait()
	log.Println("SIMULATOR: all simulations stopped")
}

// simulateStock runs the price simulation loop for a single stock
func (s *PriceSimulator) simulateStock(cfg StockConfig) {
	defer s.wg.Done()

	log.Printf("SIMULATOR: started simulation for %s (initial=%.2f, volatility=%.4f)",
		cfg.Code, cfg.InitialPrice, cfg.Volatility)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		// Random interval between ticks (500ms - 3000ms)
		interval := time.Duration(500+rng.Intn(2500)) * time.Millisecond

		select {
		case <-s.done:
			log.Printf("SIMULATOR: stopped simulation for %s", cfg.Code)
			return
		case <-time.After(interval):
			s.generateTick(cfg, rng)
		}
	}
}

// generateTick creates a single price change event
func (s *PriceSimulator) generateTick(cfg StockConfig, rng *rand.Rand) {
	md := s.marketRepo.Get(cfg.Code)
	if md == nil {
		return
	}

	currentTicker := md.GetTicker()
	currentPrice := currentTicker.LastPrice

	// Random walk: price change between -volatility% and +volatility%
	change := (rng.Float64()*2 - 1) * cfg.Volatility
	newPrice := currentPrice * (1 + change)

	// Ensure price doesn't go below minimum
	if newPrice < 1.0 {
		newPrice = 1.0
	}

	// Round to 2 decimal places
	newPrice = float64(int(newPrice*100)) / 100

	// Simulate a small volume
	volume := int64(rng.Intn(100) + 1) * 100 // 100-10000 shares

	// Update market data
	md.UpdatePrice(newPrice, volume)

	// Emit ticker event
	if s.onEvent != nil {
		s.onEvent(domain.Event{
			Type:      domain.EventTicker,
			Channel:   "market.ticker",
			StockCode: cfg.Code,
			Data:      md.GetTicker(),
		})
	}
}
