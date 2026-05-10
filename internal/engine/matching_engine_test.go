package engine

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/repository"
)

func setupEngine() (*MatchingEngine, *repository.InMemoryOrderRepository, *repository.InMemoryTradeRepository, *repository.MarketRepository) {
	orderRepo := repository.NewOrderRepository()
	tradeRepo := repository.NewTradeRepository()
	marketRepo := repository.NewMarketRepository()

	
	marketRepo.GetOrCreate("BBCA", 9500)

	var eventCount int64
	onEvent := func(event domain.Event) {
		atomic.AddInt64(&eventCount, 1)
	}

	engine := NewMatchingEngine(orderRepo, tradeRepo, marketRepo, onEvent)
	return engine, orderRepo, tradeRepo, marketRepo
}

func TestMatchingEngine_BasicMatch(t *testing.T) {
	engine, orderRepo, tradeRepo, _ := setupEngine()

	
	sell := domain.NewOrder("S1", "BBCA", domain.SideSell, 9500, 100)
	orderRepo.Save(sell)

	
	buy := domain.NewOrder("B1", "BBCA", domain.SideBuy, 9500, 100)
	orderRepo.Save(buy)
	engine.ProcessOrder(buy)

	
	trades, _ := tradeRepo.GetAll()
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if trades[0].Price != 9500 {
		t.Errorf("expected trade price 9500, got %f", trades[0].Price)
	}
	if trades[0].Quantity != 100 {
		t.Errorf("expected trade quantity 100, got %d", trades[0].Quantity)
	}

	
	if sell.GetStatus() != domain.OrderStatusFilled {
		t.Errorf("sell order should be FILLED, got %s", sell.GetStatus())
	}
	if buy.GetStatus() != domain.OrderStatusFilled {
		t.Errorf("buy order should be FILLED, got %s", buy.GetStatus())
	}
}

func TestMatchingEngine_NoMatch_PriceMismatch(t *testing.T) {
	engine, orderRepo, tradeRepo, _ := setupEngine()

	
	sell := domain.NewOrder("S1", "BBCA", domain.SideSell, 9600, 100)
	orderRepo.Save(sell)

	
	buy := domain.NewOrder("B1", "BBCA", domain.SideBuy, 9500, 100)
	orderRepo.Save(buy)
	engine.ProcessOrder(buy)

	trades, _ := tradeRepo.GetAll()
	if len(trades) != 0 {
		t.Fatalf("expected 0 trades, got %d", len(trades))
	}
}

func TestMatchingEngine_PartialFill(t *testing.T) {
	engine, orderRepo, tradeRepo, _ := setupEngine()

	
	sell := domain.NewOrder("S1", "BBCA", domain.SideSell, 9500, 50)
	orderRepo.Save(sell)

	
	buy := domain.NewOrder("B1", "BBCA", domain.SideBuy, 9500, 100)
	orderRepo.Save(buy)
	engine.ProcessOrder(buy)

	trades, _ := tradeRepo.GetAll()
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Quantity != 50 {
		t.Errorf("expected trade quantity 50, got %d", trades[0].Quantity)
	}

	
	if sell.GetStatus() != domain.OrderStatusFilled {
		t.Errorf("sell should be FILLED, got %s", sell.GetStatus())
	}

	
	if buy.GetStatus() != domain.OrderStatusPartial {
		t.Errorf("buy should be PARTIAL, got %s", buy.GetStatus())
	}
	if buy.GetRemainingQty() != 50 {
		t.Errorf("buy remaining should be 50, got %d", buy.GetRemainingQty())
	}
}

func TestMatchingEngine_FIFOMatching(t *testing.T) {
	engine, orderRepo, tradeRepo, _ := setupEngine()

	
	s1 := domain.NewOrder("S1", "BBCA", domain.SideSell, 9500, 100)
	s2 := domain.NewOrder("S2", "BBCA", domain.SideSell, 9500, 100)
	s3 := domain.NewOrder("S3", "BBCA", domain.SideSell, 9500, 100)
	orderRepo.Save(s1)
	orderRepo.Save(s2)
	orderRepo.Save(s3)

	
	buy := domain.NewOrder("B1", "BBCA", domain.SideBuy, 9500, 150)
	orderRepo.Save(buy)
	engine.ProcessOrder(buy)

	trades, _ := tradeRepo.GetAll()
	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}

	
	if s1.GetStatus() != domain.OrderStatusFilled {
		t.Errorf("S1 should be FILLED, got %s", s1.GetStatus())
	}
	
	if s2.GetStatus() != domain.OrderStatusPartial {
		t.Errorf("S2 should be PARTIAL, got %s", s2.GetStatus())
	}
	
	if s3.GetStatus() != domain.OrderStatusOpen {
		t.Errorf("S3 should be OPEN, got %s", s3.GetStatus())
	}
}

func TestMatchingEngine_ConcurrentOrders(t *testing.T) {
	engine, orderRepo, tradeRepo, _ := setupEngine()

	
	for i := 0; i < 100; i++ {
		sell := domain.NewOrder(fmt.Sprintf("S%d", i), "BBCA", domain.SideSell, 9500, 10)
		orderRepo.Save(sell)
	}

	
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			buy := domain.NewOrder(fmt.Sprintf("B%d", i), "BBCA", domain.SideBuy, 9500, 10)
			orderRepo.Save(buy)
			engine.ProcessOrder(buy)
		}(i)
	}
	wg.Wait()

	
	trades, _ := tradeRepo.GetAll()
	if len(trades) != 100 {
		t.Errorf("expected 100 trades, got %d", len(trades))
	}

	
	var totalQty int64
	for _, trade := range trades {
		totalQty += trade.Quantity
	}
	if totalQty != 1000 {
		t.Errorf("expected total quantity 1000, got %d", totalQty)
	}
}

func TestMatchingEngine_OrderBook(t *testing.T) {
	engine, orderRepo, _, _ := setupEngine()

	
	orderRepo.Save(domain.NewOrder("B1", "BBCA", domain.SideBuy, 9400, 100))
	orderRepo.Save(domain.NewOrder("B2", "BBCA", domain.SideBuy, 9400, 50))
	orderRepo.Save(domain.NewOrder("B3", "BBCA", domain.SideBuy, 9300, 200))

	
	orderRepo.Save(domain.NewOrder("S1", "BBCA", domain.SideSell, 9500, 100))
	orderRepo.Save(domain.NewOrder("S2", "BBCA", domain.SideSell, 9600, 150))

	book := engine.GetOrderBook("BBCA")

	if len(book.Bids) != 2 {
		t.Errorf("expected 2 bid levels, got %d", len(book.Bids))
	}
	if len(book.Asks) != 2 {
		t.Errorf("expected 2 ask levels, got %d", len(book.Asks))
	}

	
	if book.Bids[0].Price != 9400 {
		t.Errorf("expected first bid at 9400, got %f", book.Bids[0].Price)
	}
	if book.Bids[0].Quantity != 150 { 
		t.Errorf("expected bid qty 150, got %d", book.Bids[0].Quantity)
	}

	
	if book.Asks[0].Price != 9500 {
		t.Errorf("expected first ask at 9500, got %f", book.Asks[0].Price)
	}
}
