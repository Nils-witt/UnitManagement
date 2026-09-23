package units

import (
	"testing"

	"github.com/google/uuid"
)

func TestBrokerDeliversToSubscribers(t *testing.T) {
	t.Parallel()

	b := NewBroker()
	a, cancelA := b.Subscribe()
	defer cancelA()
	c, cancelC := b.Subscribe()
	defer cancelC()

	ev := Event{Type: EventDeleted, ID: uuid.New()}
	b.Publish(ev)
	for _, ch := range []<-chan Event{a, c} {
		if got := <-ch; got != ev {
			t.Errorf("got %+v, want %+v", got, ev)
		}
	}
}

func TestBrokerCancelClosesChannel(t *testing.T) {
	t.Parallel()

	b := NewBroker()
	ch, cancel := b.Subscribe()
	cancel()
	cancel() // idempotent
	if _, ok := <-ch; ok {
		t.Fatal("channel still open after cancel")
	}
	b.Publish(Event{Type: EventDeleted}) // must not panic on a closed channel
}

func TestBrokerDropsSlowSubscriber(t *testing.T) {
	t.Parallel()

	b := NewBroker()
	slow, cancel := b.Subscribe()
	defer cancel()
	for range subscriberBuffer + 1 {
		b.Publish(Event{Type: EventDeleted})
	}
	n := 0
	for range slow {
		n++
	}
	if n != subscriberBuffer {
		t.Errorf("received %d events before close, want %d", n, subscriberBuffer)
	}
}
