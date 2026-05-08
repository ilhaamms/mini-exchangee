package redis

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with convenience methods
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client and verifies the connection
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

// GetRDB returns the underlying redis.Client for advanced usage
func (c *Client) GetRDB() *redis.Client {
	return c.rdb
}

// Close closes the Redis connection
func (c *Client) Close() error {
	log.Println("REDIS: closing connection")
	return c.rdb.Close()
}
