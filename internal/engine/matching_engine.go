package engine

import (
	"fmt"
	"log"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/repository"
)

type EventCallback func(event domain.Event)

type MatchingEngine struct {
	mu         sync.Mutex
	stockMu    map[string]*sync.Mutex
	orderRepo  repository.OrderRepository
	tradeRepo  repository.TradeRepository
	marketRepo *repository.MarketRepository
	onEvent    EventCallback
	tradeSeq   int64
}

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

func (e *MatchingEngine) ProcessOrder(order *domain.Order) {
	
	
	stockMu := e.getStockMutex(order.StockCode)
	stockMu.Lock()
	defer stockMu.Unlock()

	var oppositeSide domain.Side
	if order.Side == domain.SideBuy {
		oppositeSide = domain.SideSell
	} else {
		oppositeSide = domain.SideBuy
	}

	
	oppositeOrders, err := e.orderRepo.GetOpenOrdersByStock(order.StockCode, oppositeSide)
	if err != nil {
		log.Printf("ERROR: failed to get open orders for %s: %v", order.StockCode, err)
		return
	}

	for _, counterOrder := range oppositeOrders {
		if order.GetRemainingQty() <= 0 {
			break
		}

		
		if !e.pricesMatch(order, counterOrder) {
			continue
		}

		
		fillQty := min(order.GetRemainingQty(), counterOrder.GetRemainingQty())
		if fillQty <= 0 {
			continue
		}

		
		tradePrice := counterOrder.Price

		
		order.Fill(fillQty)
		counterOrder.Fill(fillQty)

		
		if err := e.orderRepo.UpdateStatus(order); err != nil {
			log.Printf("ERROR: failed to update order status %s: %v", order.ID, err)
		}
		if err := e.orderRepo.UpdateStatus(counterOrder); err != nil {
			log.Printf("ERROR: failed to update order status %s: %v", counterOrder.ID, err)
		}

		
		e.tradeSeq++
		tradeID := fmt.Sprintf("T%010d", e.tradeSeq)

		
		var buyOrderID, sellOrderID string
		if order.Side == domain.SideBuy {
			buyOrderID = order.ID
			sellOrderID = counterOrder.ID
		} else {
			buyOrderID = counterOrder.ID
			sellOrderID = order.ID
		}

		
		trade := domain.NewTrade(tradeID, order.StockCode, buyOrderID, sellOrderID, tradePrice, fillQty)
		if err := e.tradeRepo.Save(trade); err != nil {
			log.Printf("ERROR: failed to save trade: %v", err)
			continue
		}

		
		md := e.marketRepo.GetOrCreate(order.StockCode, tradePrice)
		md.UpdatePrice(tradePrice, fillQty)

		
		if e.onEvent != nil {
			e.onEvent(domain.Event{
				Type:      domain.EventTrade,
				Channel:   "market.trade",
				StockCode: order.StockCode,
				Data:      trade,
			})

			
			e.onEvent(domain.Event{
				Type:      domain.EventTicker,
				Channel:   "market.ticker",
				StockCode: order.StockCode,
				Data:      md.GetTicker(),
			})

			
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

func (e *MatchingEngine) pricesMatch(incoming, resting *domain.Order) bool {
	if incoming.Side == domain.SideBuy {
		
		return incoming.Price >= resting.Price
	}
	
	return incoming.Price <= resting.Price
}

func (e *MatchingEngine) GetOrderBook(stockCode string) domain.OrderBook {
	stockMu := e.getStockMutex(stockCode)
	stockMu.Lock()
	defer stockMu.Unlock()

	buyOrders, _ := e.orderRepo.GetOpenOrdersByStock(stockCode, domain.SideBuy)
	sellOrders, _ := e.orderRepo.GetOpenOrdersByStock(stockCode, domain.SideSell)

	
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

	
	bids := make([]domain.OrderBookEntry, 0, len(bidMap))
	for _, entry := range bidMap {
		bids = append(bids, *entry)
	}
	
	sortDesc := func(i, j int) bool { return bids[i].Price > bids[j].Price }

	asks := make([]domain.OrderBookEntry, 0, len(askMap))
	for _, entry := range askMap {
		asks = append(asks, *entry)
	}
	
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
