package redis

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(addr, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("REDIS: connected successfully")
	return &Client{rdb: rdb}, nil
}

func (c *Client) GetRDB() *redis.Client {
	return c.rdb
}

func (c *Client) Close() error {
	log.Println("REDIS: closing connection")
	return c.rdb.Close()
}
