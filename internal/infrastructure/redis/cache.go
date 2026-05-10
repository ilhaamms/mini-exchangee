package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ilhaamms/ybtech/internal/domain"
)

type Cache struct {
	client *Client
}

func NewCache(client *Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) SetTicker(ctx context.Context, ticker domain.Ticker) error {
	key := fmt.Sprintf("ticker:%s", ticker.StockCode)
	data, err := json.Marshal(ticker)
	if err != nil {
		return err
	}
	return c.client.GetRDB().Set(ctx, key, data, 5*time.Second).Err()
}

func (c *Cache) GetTicker(ctx context.Context, stockCode string) (*domain.Ticker, error) {
	key := fmt.Sprintf("ticker:%s", stockCode)
	data, err := c.client.GetRDB().Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var ticker domain.Ticker
	if err := json.Unmarshal(data, &ticker); err != nil {
		return nil, err
	}
	return &ticker, nil
}

func (c *Cache) SetOrderBook(ctx context.Context, book domain.OrderBook) error {
	key := fmt.Sprintf("orderbook:%s", book.StockCode)
	data, err := json.Marshal(book)
	if err != nil {
		return err
	}
	return c.client.GetRDB().Set(ctx, key, data, 2*time.Second).Err()
}

func (c *Cache) GetOrderBook(ctx context.Context, stockCode string) (*domain.OrderBook, error) {
	key := fmt.Sprintf("orderbook:%s", stockCode)
	data, err := c.client.GetRDB().Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var book domain.OrderBook
	if err := json.Unmarshal(data, &book); err != nil {
		return nil, err
	}
	return &book, nil
}

func (c *Cache) AddRecentTrade(ctx context.Context, trade *domain.Trade) error {
	key := fmt.Sprintf("trades:recent:%s", trade.StockCode)
	data, err := json.Marshal(trade)
	if err != nil {
		return err
	}

	pipe := c.client.GetRDB().Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, 99) 
	pipe.Expire(ctx, key, 24*time.Hour)
	_, err = pipe.Exec(ctx)

	if err != nil {
		log.Printf("REDIS_CACHE: failed to cache trade: %v", err)
	}
	return err
}
