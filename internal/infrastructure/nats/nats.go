package nats

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/ilhaamms/ybtech/internal/domain"
)

const (
	// SubjectTradeEvent is the NATS subject for trade events
	SubjectTradeEvent = "exchange.events.trade"
	// SubjectTickerEvent is the NATS subject for ticker events
	SubjectTickerEvent = "exchange.events.ticker"
	// SubjectOrderEvent is the NATS subject for order update events
	SubjectOrderEvent = "exchange.events.order"
	// SubjectAllEvents is the NATS subject for all events (wildcard)
	SubjectAllEvents = "exchange.events.*"
)

// Broker wraps a NATS connection and provides pub/sub functionality.
//
// Architecture:
//   MatchingEngine --> NATS Publisher --> "exchange.events.*"
//                                              |
//                                    NATS Subscriber (worker)
//                                              |
//                                    WebSocket Hub.BroadcastEvent()
//
// This decouples the matching engine from the delivery layer,
// allowing independent scaling of matching and broadcasting.
type Broker struct {
	conn *nats.Conn
	subs []*nats.Subscription
	mu   sync.Mutex
}

// NewBroker creates a new NATS broker connection
func NewBroker(url string) (*Broker, error) {
	conn, err := nats.Connect(url,
		nats.Name("mini-exchange"),
		nats.ReconnectWait(nats.DefaultReconnectWait),
		nats.MaxReconnects(-1), // unlimited reconnects
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

// Publish sends an event to the appropriate NATS subject
func (b *Broker) Publish(event domain.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	subject := eventTypeToSubject(event.Type)
	return b.conn.Publish(subject, data)
}

// Subscribe registers a handler for all exchange events.
// The onEvent callback is invoked for each received event.
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

// Close gracefully shuts down the NATS connection
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subs {
		sub.Unsubscribe()
	}

	b.conn.Drain()
	log.Println("NATS: connection closed")
}

// eventTypeToSubject maps domain event types to NATS subjects
func eventTypeToSubject(t domain.EventType) string {
	switch t {
	case domain.EventTrade:
		return SubjectTradeEvent
	case domain.EventTicker:
		return SubjectTickerEvent
	case domain.EventOrderUpdate:
		return SubjectOrderEvent
	default:
		return SubjectAllEvents
	}
}
