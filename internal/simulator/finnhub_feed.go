package simulator

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/repository"
)

// FinnhubSymbolMap maps Finnhub symbol strings to internal stock codes.
// Finnhub uses "BINANCE:BTCUSDT" for crypto, or "AAPL" for US stocks.
var FinnhubSymbolMap = map[string]string{
	"BINANCE:BTCUSDT": "BTCUSDT",
	"BINANCE:ETHUSDT": "ETHUSDT",
	"BINANCE:BNBUSDT": "BNBUSDT",
	"BINANCE:SOLUSDT": "SOLUSDT",
	"BINANCE:ADAUSDT": "ADAUSDT",
}

// finnhubSubscribeMsg is the JSON message sent to subscribe to a symbol.
type finnhubSubscribeMsg struct {
	Type   string `json:"type"`
	Symbol string `json:"symbol"`
}

// finnhubTradeMsg is the top-level message received from Finnhub WS.
type finnhubTradeMsg struct {
	Type string             `json:"type"`
	Data []finnhubTradeItem `json:"data"`
}

// finnhubTradeItem is one trade record inside a trade message.
type finnhubTradeItem struct {
	Symbol    string  `json:"s"` // Finnhub symbol, e.g. "BINANCE:BTCUSDT"
	Price     float64 `json:"p"` // Last trade price
	Volume    float64 `json:"v"` // Trade volume
	Timestamp int64   `json:"t"` // Unix milliseconds
}

// FinnhubFeed streams real market data from Finnhub WebSocket API.
// Docs: https://finnhub.io/docs/api/websocket-trades
type FinnhubFeed struct {
	apiKey     string
	marketRepo *repository.MarketRepository
	onEvent    func(event domain.Event)
	conn       *ws.Conn
	connMu     sync.Mutex
	done       chan struct{}
	wg         sync.WaitGroup
}

// NewFinnhubFeed creates a FinnhubFeed. apiKey must not be empty.
func NewFinnhubFeed(
	apiKey string,
	marketRepo *repository.MarketRepository,
	onEvent func(event domain.Event),
) *FinnhubFeed {
	return &FinnhubFeed{
		apiKey:     apiKey,
		marketRepo: marketRepo,
		onEvent:    onEvent,
		done:       make(chan struct{}),
	}
}

// Start connects to Finnhub WebSocket and begins streaming.
func (f *FinnhubFeed) Start() error {
	url := "wss://ws.finnhub.io?token=" + f.apiKey

	slog.Info("FINNHUB: connecting", "url", "wss://ws.finnhub.io")

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	f.connMu.Lock()
	f.conn = conn
	f.connMu.Unlock()

	// Subscribe to all configured symbols.
	for finnhubSymbol := range FinnhubSymbolMap {
		msg := finnhubSubscribeMsg{Type: "subscribe", Symbol: finnhubSymbol}
		if err := conn.WriteJSON(msg); err != nil {
			conn.Close()
			return err
		}
	}

	// Pre-create market data entries with price 0 (will be updated on first tick).
	for _, stockCode := range FinnhubSymbolMap {
		f.marketRepo.GetOrCreate(stockCode, 0)
	}

	slog.Info("FINNHUB: connected, streaming real market data", "symbols", len(FinnhubSymbolMap))

	f.wg.Add(1)
	go f.readLoop()

	return nil
}

func (f *FinnhubFeed) readLoop() {
	defer f.wg.Done()

	for {
		select {
		case <-f.done:
			return
		default:
			f.connMu.Lock()
			conn := f.conn
			f.connMu.Unlock()

			_, message, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-f.done:
					return
				default:
				}
				slog.Warn("FINNHUB: read error, attempting reconnect", "error", err)
				select {
				case <-f.done:
					return
				case <-time.After(5 * time.Second):
				}
				if err := f.reconnect(); err != nil {
					slog.Error("FINNHUB: reconnect failed", "error", err)
					return
				}
				continue
			}

			f.handleMessage(message)
		}
	}
}

func (f *FinnhubFeed) handleMessage(message []byte) {
	var msg finnhubTradeMsg
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	// Finnhub sends "ping" keepalive and "trade" events.
	if msg.Type != "trade" {
		return
	}

	for _, item := range msg.Data {
		stockCode, ok := FinnhubSymbolMap[item.Symbol]
		if !ok {
			continue
		}
		if item.Price <= 0 {
			continue
		}

		md := f.marketRepo.GetOrCreate(stockCode, item.Price)
		md.UpdatePrice(item.Price, int64(item.Volume))

		if f.onEvent != nil {
			f.onEvent(domain.Event{
				Type:      domain.EventTicker,
				Channel:   "market.ticker",
				StockCode: stockCode,
				Data:      md.GetTicker(),
			})
		}
	}
}

func (f *FinnhubFeed) reconnect() error {
	url := "wss://ws.finnhub.io?token=" + f.apiKey

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	for finnhubSymbol := range FinnhubSymbolMap {
		msg := finnhubSubscribeMsg{Type: "subscribe", Symbol: finnhubSymbol}
		if err := conn.WriteJSON(msg); err != nil {
			conn.Close()
			return err
		}
	}

	f.connMu.Lock()
	f.conn = conn
	f.connMu.Unlock()
	slog.Info("FINNHUB: reconnected successfully")
	return nil
}

// Stop gracefully shuts down the feed.
func (f *FinnhubFeed) Stop() {
	slog.Info("FINNHUB: stopping feed")
	close(f.done)
	f.connMu.Lock()
	if f.conn != nil {
		f.conn.Close()
	}
	f.connMu.Unlock()
	f.wg.Wait()
	slog.Info("FINNHUB: feed stopped")
}
