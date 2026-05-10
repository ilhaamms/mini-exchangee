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

var BinanceSymbolMap = map[string]string{
	"BTCUSDT": "btcusdt",
	"ETHUSDT": "ethusdt",
	"BNBUSDT": "bnbusdt",
	"SOLUSDT": "solusdt",
	"ADAUSDT": "adausdt",
}

type BinanceTicker struct {
	EventType string `json:"e"` 
	Symbol    string `json:"s"` 
	Close     string `json:"c"` 
	Open      string `json:"o"` 
	High      string `json:"h"` 
	Low       string `json:"l"` 
	Volume    string `json:"v"` 
}

type BinanceFeed struct {
	marketRepo *repository.MarketRepository
	onEvent    func(event domain.Event)
	conn       *ws.Conn
	done       chan struct{}
	wg         sync.WaitGroup
}

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

func (f *BinanceFeed) Start() error {
	
	
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

	
	for stockCode := range BinanceSymbolMap {
		f.marketRepo.GetOrCreate(stockCode, 0)
	}

	f.wg.Add(1)
	go f.readLoop()

	return nil
}

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

func (f *BinanceFeed) handleMessage(message []byte) {
	
	var wrapper struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &wrapper); err != nil {
		
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

func (f *BinanceFeed) processTicker(ticker BinanceTicker) {
	stockCode := ticker.Symbol

	
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

func (f *BinanceFeed) Stop() {
	log.Println("BINANCE: stopping feed")
	close(f.done)
	if f.conn != nil {
		f.conn.Close()
	}
	f.wg.Wait()
	log.Println("BINANCE: feed stopped")
}

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
