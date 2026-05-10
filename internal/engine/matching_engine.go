package engine

import (
	"fmt"
	"log"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/repository"
)

// EventCallback is called when a trade occurs or order status changes
type EventCallback func(event domain.Event)

// MatchingEngine implements a simple price-time priority (FIFO) matching engine.
//
// Concurrency Strategy:
// - Each stock has its own mutex (stockMu) to allow parallel matching across stocks
// - Orders within the same stock are processed sequentially to prevent race conditions
// - This design allows ~1000 orders/min as matching for BBCA doesn't block TLKM
//
// Race Condition Prevention:
// - Stock-level locking ensures only one goroutine matches orders for a given stock at a time
// - Order.Fill() uses its own internal mutex for field updates
// - Repository operations are also internally synchronized
type MatchingEngine struct {
	mu         sync.Mutex
	stockMu    map[string]*sync.Mutex
	orderRepo  repository.OrderRepository
	tradeRepo  repository.TradeRepository
	marketRepo *repository.MarketRepository
	onEvent    EventCallback
	tradeSeq   int64
}

// NewMatchingEngine creates a new matching engine
func NewMatchingEngine(
	orderRepo repository.OrderRepository,
	tradeRepo repository.TradeRepository,
	marketRepo *repository.MarketRepository,
	onEvent EventCallback,
) *MatchingEngine {
	return &MatchingEngine{
		stockMu:    make(map[string]*sync.Mutex),
		orderRepo:  orderRepo,
		tradeRepo:  tradeRepo,
		marketRepo: marketRepo,
		onEvent:    onEvent,
		tradeSeq:   0,
	}
}

// getStockMutex returns or creates a mutex for a specific stock
func (e *MatchingEngine) getStockMutex(stockCode string) *sync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()

	if m, ok := e.stockMu[stockCode]; ok {
		return m
	}
	m := &sync.Mutex{}
	e.stockMu[stockCode] = m
	return m
}

// ProcessOrder attempts to match a new order against existing orders
// It follows FIFO price-time priority:
// - BUY orders match against SELL orders at or below the buy price
// - SELL orders match against BUY orders at or above the sell price
// - Within the same price level, earlier orders are matched first (FIFO)
func (e *MatchingEngine) ProcessOrder(order *domain.Order) {
	// Lock per-stock to prevent race conditions within the same stock
	// while allowing concurrent matching across different stocks
	stockMu := e.getStockMutex(order.StockCode)
	stockMu.Lock()
	defer stockMu.Unlock()

	var oppositeSide domain.Side
	if order.Side == domain.SideBuy {
		oppositeSide = domain.SideSell
	} else {
		oppositeSide = domain.SideBuy
	}

	// Get opposite side orders sorted by FIFO (creation time)
	oppositeOrders, err := e.orderRepo.GetOpenOrdersByStock(order.StockCode, oppositeSide)
	if err != nil {
		log.Printf("ERROR: failed to get open orders for %s: %v", order.StockCode, err)
		return
	}

	for _, counterOrder := range oppositeOrders {
		if order.GetRemainingQty() <= 0 {
			break
		}

		// Check price compatibility
		if !e.pricesMatch(order, counterOrder) {
			continue
		}

		// Determine fill quantity (partial fill support)
		fillQty := min(order.GetRemainingQty(), counterOrder.GetRemainingQty())
		if fillQty <= 0 {
			continue
		}

		// Determine trade price (use the resting order's price - price-time priority)
		tradePrice := counterOrder.Price

		// Execute the fill on both orders
		order.Fill(fillQty)
		counterOrder.Fill(fillQty)

		// Persist the updated order statuses (no-op for in-memory, actual UPDATE for postgres)
		if err := e.orderRepo.UpdateStatus(order); err != nil {
			log.Printf("ERROR: failed to update order status %s: %v", order.ID, err)
		}
		if err := e.orderRepo.UpdateStatus(counterOrder); err != nil {
			log.Printf("ERROR: failed to update order status %s: %v", counterOrder.ID, err)
		}

		// Generate trade ID
		e.tradeSeq++
		tradeID := fmt.Sprintf("T%010d", e.tradeSeq)

		// Determine buy/sell order IDs
		var buyOrderID, sellOrderID string
		if order.Side == domain.SideBuy {
			buyOrderID = order.ID
			sellOrderID = counterOrder.ID
		} else {
			buyOrderID = counterOrder.ID
			sellOrderID = order.ID
		}

		// Create and save trade
		trade := domain.NewTrade(tradeID, order.StockCode, buyOrderID, sellOrderID, tradePrice, fillQty)
		if err := e.tradeRepo.Save(trade); err != nil {
			log.Printf("ERROR: failed to save trade: %v", err)
			continue
		}

		// Update market data
		md := e.marketRepo.GetOrCreate(order.StockCode, tradePrice)
		md.UpdatePrice(tradePrice, fillQty)

		// Emit trade event
		if e.onEvent != nil {
			e.onEvent(domain.Event{
				Type:      domain.EventTrade,
				Channel:   "market.trade",
				StockCode: order.StockCode,
				Data:      trade,
			})

			// Emit ticker update
			e.onEvent(domain.Event{
				Type:      domain.EventTicker,
				Channel:   "market.ticker",
				StockCode: order.StockCode,
				Data:      md.GetTicker(),
			})

			// Emit order update for both orders
			e.onEvent(domain.Event{
				Type:      domain.EventOrderUpdate,
				Channel:   "order.update",
				StockCode: order.StockCode,
				Data:      order.ToSnapshot(),
			})
			e.onEvent(domain.Event{
				Type:      domain.EventOrderUpdate,
				Channel:   "order.update",
				StockCode: counterOrder.StockCode,
				Data:      counterOrder.ToSnapshot(),
			})
		}

		log.Printf("TRADE: %s %s %d @ %.2f (buy=%s, sell=%s)",
			tradeID, order.StockCode, fillQty, tradePrice, buyOrderID, sellOrderID)
	}
}

// pricesMatch checks if two orders can be matched based on price
func (e *MatchingEngine) pricesMatch(incoming, resting *domain.Order) bool {
	if incoming.Side == domain.SideBuy {
		// Buy order matches sell if buy price >= sell price
		return incoming.Price >= resting.Price
	}
	// Sell order matches buy if sell price <= buy price
	return incoming.Price <= resting.Price
}

// GetOrderBook builds the current order book (bid/ask depth) for a stock
func (e *MatchingEngine) GetOrderBook(stockCode string) domain.OrderBook {
	stockMu := e.getStockMutex(stockCode)
	stockMu.Lock()
	defer stockMu.Unlock()

	buyOrders, _ := e.orderRepo.GetOpenOrdersByStock(stockCode, domain.SideBuy)
	sellOrders, _ := e.orderRepo.GetOpenOrdersByStock(stockCode, domain.SideSell)

	// Aggregate bids by price level
	bidMap := make(map[float64]*domain.OrderBookEntry)
	for _, o := range buyOrders {
		snap := o.ToSnapshot()
		if entry, ok := bidMap[snap.Price]; ok {
			entry.Quantity += snap.RemainingQty
			entry.Count++
		} else {
			bidMap[snap.Price] = &domain.OrderBookEntry{
				Price:    snap.Price,
				Quantity: snap.RemainingQty,
				Count:    1,
			}
		}
	}

	// Aggregate asks by price level
	askMap := make(map[float64]*domain.OrderBookEntry)
	for _, o := range sellOrders {
		snap := o.ToSnapshot()
		if entry, ok := askMap[snap.Price]; ok {
			entry.Quantity += snap.RemainingQty
			entry.Count++
		} else {
			askMap[snap.Price] = &domain.OrderBookEntry{
				Price:    snap.Price,
				Quantity: snap.RemainingQty,
				Count:    1,
			}
		}
	}

	// Convert to slices and sort
	bids := make([]domain.OrderBookEntry, 0, len(bidMap))
	for _, entry := range bidMap {
		bids = append(bids, *entry)
	}
	// Sort bids descending by price (highest first)
	sortDesc := func(i, j int) bool { return bids[i].Price > bids[j].Price }

	asks := make([]domain.OrderBookEntry, 0, len(askMap))
	for _, entry := range askMap {
		asks = append(asks, *entry)
	}
	// Sort asks ascending by price (lowest first)
	sortAsc := func(i, j int) bool { return asks[i].Price < asks[j].Price }

	sortSlice(bids, sortDesc)
	sortSlice(asks, sortAsc)

	return domain.OrderBook{
		StockCode: stockCode,
		Bids:      bids,
		Asks:      asks,
	}
}

func sortSlice(entries []domain.OrderBookEntry, less func(i, j int) bool) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}
