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
	
	EventChannel = "mini-exchange:events"
)

type PubSub struct {
	client     *Client
	subscriber *goredis.PubSub
	onEvent    func(event domain.Event) 
	done       chan struct{}
	wg         sync.WaitGroup
}

func NewPubSub(client *Client, onEvent func(event domain.Event)) *PubSub {
	return &PubSub{
		client:  client,
		onEvent: onEvent,
		done:    make(chan struct{}),
	}
}

func (ps *PubSub) Start(ctx context.Context) error {
	ps.subscriber = ps.client.GetRDB().Subscribe(ctx, EventChannel)

	
	_, err := ps.subscriber.Receive(ctx)
	if err != nil {
		return err
	}

	log.Printf("REDIS_PUBSUB: subscribed to channel '%s'", EventChannel)

	ps.wg.Add(1)
	go ps.listenLoop()

	return nil
}

func (ps *PubSub) Publish(ctx context.Context, event domain.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return ps.client.GetRDB().Publish(ctx, EventChannel, data).Err()
}

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

func (ps *PubSub) Stop() {
	log.Println("REDIS_PUBSUB: stopping")
	close(ps.done)
	if ps.subscriber != nil {
		ps.subscriber.Close()
	}
	ps.wg.Wait()
	log.Println("REDIS_PUBSUB: stopped")
}
