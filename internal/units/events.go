package units

import (
	"sync"

	"github.com/google/uuid"

	"go-unit-mangement/internal/models"
)

type EventType string

const (
	EventCreated EventType = "created"
	EventUpdated EventType = "updated"
	EventDeleted EventType = "deleted"
)

// Event describes a change to a unit. Unit is nil for EventDeleted.
type Event struct {
	Type EventType
	ID   uuid.UUID
	Unit *models.Unit
}

// subscriberBuffer is how many events a subscriber may fall behind before it
// is dropped.
const subscriberBuffer = 64

// Broker fans unit events out to subscribers.
type Broker struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{subs: make(map[chan Event]struct{})}
}

// Subscribe returns a channel of events and a function that ends the
// subscription. The channel is closed when the subscription ends, either by
// calling cancel or because the subscriber fell too far behind; a subscriber
// whose channel closes on its own has missed events and should resync.
func (b *Broker) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.remove(ch)
	}
}

// Publish sends ev to every subscriber without blocking.
func (b *Broker) Publish(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			b.remove(ch)
		}
	}
}

// remove must be called with mu held.
func (b *Broker) remove(ch chan Event) {
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
}
