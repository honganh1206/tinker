// Package eventbus provides different implementations of event bus (local, NATS JetStream, etc.)
package eventbus

import (
	"context"
	"sync"
)

type LocalEventBus struct {
	mu   sync.RWMutex
	subs map[string][]chan *Event
}

func NewLocalEventBus() (*LocalEventBus, error) {
	return &LocalEventBus{subs: make(map[string][]chan *Event)}, nil
}

// Publish sends event to the Event channel of all subscriptions.
func (b *LocalEventBus) Publish(ctx context.Context, topic string, event *Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[topic] {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

// Subscribe appends a subscription to a topic
func (b *LocalEventBus) Subscribe(ctx context.Context, topic string) (<-chan *Event, error) {
	// TODO: Why limit to 64?
	ch := make(chan *Event, 64)
	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], ch)
	b.mu.Unlock()
	return ch, nil
}

func (b *LocalEventBus) Close() error { return nil }
