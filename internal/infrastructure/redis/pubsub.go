package redis

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	goredis "github.com/redis/go-redis/v9"
	"github.com/ilhaamms/ybtech/internal/domain"
)

const (
	// EventChannel is the Redis Pub/Sub channel for cross-node event broadcasting
	EventChannel = "mini-exchange:events"
)

// PubSub handles Redis Pub/Sub for horizontal scaling of WebSocket events.
//
// How it works:
// - When an event occurs (trade, ticker update), it is published to a Redis channel
// - All server instances subscribe to the same channel
// - Each instance broadcasts received events to its local WebSocket clients
// - This enables horizontal scaling: events from Node 1 reach clients on Node 2
type PubSub struct {
	client     *Client
	subscriber *goredis.PubSub
	onEvent    func(event domain.Event) // callback for received events
	done       chan struct{}
	wg         sync.WaitGroup
}

// NewPubSub creates a new Redis Pub/Sub handler
func NewPubSub(client *Client, onEvent func(event domain.Event)) *PubSub {
	return &PubSub{
		client:  client,
		onEvent: onEvent,
		done:    make(chan struct{}),
	}
}

// Start begins subscribing to the event channel
func (ps *PubSub) Start(ctx context.Context) error {
	ps.subscriber = ps.client.GetRDB().Subscribe(ctx, EventChannel)

	// Wait for subscription confirmation
	_, err := ps.subscriber.Receive(ctx)
	if err != nil {
		return err
	}

	log.Printf("REDIS_PUBSUB: subscribed to channel '%s'", EventChannel)

	ps.wg.Add(1)
	go ps.listenLoop()

	return nil
}

// Publish sends an event to all server instances via Redis
func (ps *PubSub) Publish(ctx context.Context, event domain.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return ps.client.GetRDB().Publish(ctx, EventChannel, data).Err()
}

// listenLoop reads messages from the Redis Pub/Sub channel
func (ps *PubSub) listenLoop() {
	defer ps.wg.Done()

	ch := ps.subscriber.Channel()

	for {
		select {
		case <-ps.done:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			var event domain.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("REDIS_PUBSUB: failed to unmarshal event: %v", err)
				continue
			}

			if ps.onEvent != nil {
				ps.onEvent(event)
			}
		}
	}
}

// Stop closes the Pub/Sub subscription
func (ps *PubSub) Stop() {
	log.Println("REDIS_PUBSUB: stopping")
	close(ps.done)
	if ps.subscriber != nil {
		ps.subscriber.Close()
	}
	ps.wg.Wait()
	log.Println("REDIS_PUBSUB: stopped")
}
