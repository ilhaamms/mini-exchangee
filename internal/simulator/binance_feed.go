package simulator

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/repository"
)

// BinanceSymbolMap maps internal stock codes to Binance trading pairs
var BinanceSymbolMap = map[string]string{
	"BTCUSDT": "btcusdt",
	"ETHUSDT": "ethusdt",
	"BNBUSDT": "bnbusdt",
	"SOLUSDT": "solusdt",
	"ADAUSDT": "adausdt",
}

// BinanceTicker represents a Binance mini-ticker payload
type BinanceTicker struct {
	EventType string `json:"e"` // "24hrMiniTicker"
	Symbol    string `json:"s"` // "BTCUSDT"
	Close     string `json:"c"` // current close price
	Open      string `json:"o"` // open price
	High      string `json:"h"` // high price
	Low       string `json:"l"` // low price
	Volume    string `json:"v"` // total traded base asset volume
}

// BinanceFeed connects to the Binance public WebSocket and streams
// real market data instead of using the price simulator.
//
// It subscribes to the Binance mini-ticker stream for multiple symbols
// and converts the data into the internal domain.Event format.
//
// Usage:
//   BINANCE_ENABLED=true go run cmd/server/main.go
//
// Note: This uses crypto trading pairs (BTC/ETH/etc.) instead of
// Indonesian stocks since Binance doesn't list IDX equities.
type BinanceFeed struct {
	marketRepo *repository.MarketRepository
	onEvent    func(event domain.Event)
	conn       *ws.Conn
	done       chan struct{}
	wg         sync.WaitGroup
}

// NewBinanceFeed creates a new Binance market data feed
func NewBinanceFeed(
	marketRepo *repository.MarketRepository,
	onEvent func(event domain.Event),
) *BinanceFeed {
	return &BinanceFeed{
		marketRepo: marketRepo,
		onEvent:    onEvent,
		done:       make(chan struct{}),
	}
}

// Start connects to Binance WebSocket and begins streaming data
func (f *BinanceFeed) Start() error {
	// Build combined stream URL
	// Format: wss://stream.binance.com:9443/ws/btcusdt@miniTicker/ethusdt@miniTicker/...
	var streams []string
	for _, binanceSymbol := range BinanceSymbolMap {
		streams = append(streams, binanceSymbol+"@miniTicker")
	}

	url := "wss://stream.binance.com:9443/stream?streams=" + strings.Join(streams, "/")

	log.Printf("BINANCE: connecting to %s", url)

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	f.conn = conn
	log.Println("BINANCE: connected successfully, streaming real market data")

	// Initialize market data for all symbols
	for stockCode := range BinanceSymbolMap {
		f.marketRepo.GetOrCreate(stockCode, 0)
	}

	f.wg.Add(1)
	go f.readLoop()

	return nil
}

// readLoop continuously reads messages from the Binance WebSocket
func (f *BinanceFeed) readLoop() {
	defer f.wg.Done()
	defer f.conn.Close()

	for {
		select {
		case <-f.done:
			return
		default:
			_, message, err := f.conn.ReadMessage()
			if err != nil {
				log.Printf("BINANCE: read error: %v", err)
				// Try to reconnect after delay
				time.Sleep(5 * time.Second)
				if err := f.reconnect(); err != nil {
					log.Printf("BINANCE: reconnect failed: %v", err)
					return
				}
				continue
			}

			f.handleMessage(message)
		}
	}
}

// handleMessage processes a single Binance WebSocket message
func (f *BinanceFeed) handleMessage(message []byte) {
	// Binance combined stream format: {"stream":"btcusdt@miniTicker","data":{...}}
	var wrapper struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &wrapper); err != nil {
		// Try direct ticker format
		var ticker BinanceTicker
		if err := json.Unmarshal(message, &ticker); err != nil {
			return
		}
		f.processTicker(ticker)
		return
	}

	var ticker BinanceTicker
	if err := json.Unmarshal(wrapper.Data, &ticker); err != nil {
		return
	}

	f.processTicker(ticker)
}

// processTicker converts a Binance ticker to an internal event
func (f *BinanceFeed) processTicker(ticker BinanceTicker) {
	stockCode := ticker.Symbol

	// Parse price
	var price float64
	if _, err := parseFloat(ticker.Close, &price); err != nil {
		return
	}

	var volume float64
	parseFloat(ticker.Volume, &volume)

	md := f.marketRepo.GetOrCreate(stockCode, price)
	md.UpdatePrice(price, int64(volume))

	if f.onEvent != nil {
		f.onEvent(domain.Event{
			Type:      domain.EventTicker,
			Channel:   "market.ticker",
			StockCode: stockCode,
			Data:      md.GetTicker(),
		})
	}
}

// reconnect attempts to re-establish the Binance WebSocket connection
func (f *BinanceFeed) reconnect() error {
	var streams []string
	for _, binanceSymbol := range BinanceSymbolMap {
		streams = append(streams, binanceSymbol+"@miniTicker")
	}

	url := "wss://stream.binance.com:9443/stream?streams=" + strings.Join(streams, "/")

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	f.conn = conn
	log.Println("BINANCE: reconnected successfully")
	return nil
}

// Stop gracefully shuts down the Binance feed
func (f *BinanceFeed) Stop() {
	log.Println("BINANCE: stopping feed")
	close(f.done)
	if f.conn != nil {
		f.conn.Close()
	}
	f.wg.Wait()
	log.Println("BINANCE: feed stopped")
}

// parseFloat is a helper to parse string to float64
func parseFloat(s string, f *float64) (bool, error) {
	if s == "" {
		return false, nil
	}
	var val float64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			div := float64(1)
			for i++; i < len(s); i++ {
				div *= 10
				val += float64(s[i]-'0') / div
			}
			break
		}
		val = val*10 + float64(c-'0')
	}
	*f = val
	return true, nil
}
