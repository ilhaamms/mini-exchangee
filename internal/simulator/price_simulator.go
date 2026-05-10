package simulator

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/repository"
)

type EventCallback func(event domain.Event)

type StockConfig struct {
	Code         string
	InitialPrice float64
	Volatility   float64 
}

type PriceSimulator struct {
	marketRepo *repository.MarketRepository
	onEvent    EventCallback
	configs    []StockConfig
	done       chan struct{}
	wg         sync.WaitGroup
}

func DefaultStocks() []StockConfig {
	return []StockConfig{
		{Code: "BBCA", InitialPrice: 9500.0, Volatility: 0.003},
		{Code: "BBRI", InitialPrice: 5200.0, Volatility: 0.004},
		{Code: "TLKM", InitialPrice: 3800.0, Volatility: 0.005},
		{Code: "ASII", InitialPrice: 6100.0, Volatility: 0.003},
		{Code: "BMRI", InitialPrice: 6800.0, Volatility: 0.004},
	}
}

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

func (s *PriceSimulator) Start() {
	log.Printf("SIMULATOR: starting price simulation for %d stocks", len(s.configs))

	
	for _, cfg := range s.configs {
		s.marketRepo.GetOrCreate(cfg.Code, cfg.InitialPrice)
	}

	
	for _, cfg := range s.configs {
		s.wg.Add(1)
		go s.simulateStock(cfg)
	}
}

func (s *PriceSimulator) Stop() {
	log.Println("SIMULATOR: stopping price simulation")
	close(s.done)
	s.wg.Wait()
	log.Println("SIMULATOR: all simulations stopped")
}

func (s *PriceSimulator) simulateStock(cfg StockConfig) {
	defer s.wg.Done()

	log.Printf("SIMULATOR: started simulation for %s (initial=%.2f, volatility=%.4f)",
		cfg.Code, cfg.InitialPrice, cfg.Volatility)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		
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

func (s *PriceSimulator) generateTick(cfg StockConfig, rng *rand.Rand) {
	md := s.marketRepo.Get(cfg.Code)
	if md == nil {
		return
	}

	currentTicker := md.GetTicker()
	currentPrice := currentTicker.LastPrice

	
	change := (rng.Float64()*2 - 1) * cfg.Volatility
	newPrice := currentPrice * (1 + change)

	
	if newPrice < 1.0 {
		newPrice = 1.0
	}

	
	newPrice = float64(int(newPrice*100)) / 100

	
	volume := int64(rng.Intn(100) + 1) * 100 

	
	md.UpdatePrice(newPrice, volume)

	
	if s.onEvent != nil {
		s.onEvent(domain.Event{
			Type:      domain.EventTicker,
			Channel:   "market.ticker",
			StockCode: cfg.Code,
			Data:      md.GetTicker(),
		})
	}
}
