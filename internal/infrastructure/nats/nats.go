package nats

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/nats-io/nats.go"
)

const (
	SubjectTradeEvent     = "exchange.events.trade"
	SubjectTickerEvent    = "exchange.events.ticker"
	SubjectOrderEvent     = "exchange.events.order"
	SubjectOrderBookEvent = "exchange.events.orderbook"
	SubjectAllEvents      = "exchange.events.*"
)

type Broker struct {
	conn *nats.Conn
	subs []*nats.Subscription
	mu   sync.Mutex
}

func NewBroker(url string) (*Broker, error) {
	conn, err := nats.Connect(url,
		nats.Name("mini-exchange"),
		nats.ReconnectWait(nats.DefaultReconnectWait),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("NATS: disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("NATS: reconnected to %s", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, err
	}

	log.Printf("NATS: connected to %s", conn.ConnectedUrl())
	return &Broker{conn: conn}, nil
}

func (b *Broker) Publish(event domain.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	subject := eventTypeToSubject(event.Type)
	return b.conn.Publish(subject, data)
}

func (b *Broker) Subscribe(onEvent func(event domain.Event)) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub, err := b.conn.Subscribe(SubjectAllEvents, func(msg *nats.Msg) {
		var event domain.Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			log.Printf("NATS: failed to unmarshal event: %v", err)
			return
		}
		onEvent(event)
	})
	if err != nil {
		return err
	}

	b.subs = append(b.subs, sub)
	log.Printf("NATS: subscribed to %s", SubjectAllEvents)
	return nil
}

func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subs {
		sub.Unsubscribe()
	}

	b.conn.Drain()
	log.Println("NATS: connection closed")
}

func eventTypeToSubject(t domain.EventType) string {
	switch t {
	case domain.EventTrade:
		return SubjectTradeEvent
	case domain.EventTicker:
		return SubjectTickerEvent
	case domain.EventOrderUpdate:
		return SubjectOrderEvent
	case domain.EventOrderBook:
		return SubjectOrderBookEvent
	default:
		return SubjectAllEvents
	}
}
