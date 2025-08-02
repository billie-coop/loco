package events

import (
	"sync"
)

// Subscriber receives events from the broker
type Subscriber interface {
	HandleEvent(event Event) bool // returns true if event was handled
}

// Broker manages event distribution
type Broker struct {
	subscribers map[EventType][]chan Event
	mu          sync.RWMutex
	bufferSize  int
}

// NewBroker creates a new event broker
func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[EventType][]chan Event),
		bufferSize:  10,
	}
}

// Subscribe creates a subscription to specific event types
func (b *Broker) Subscribe(eventTypes ...EventType) <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, b.bufferSize)

	// If no specific types provided, subscribe to all
	if len(eventTypes) == 0 {
		eventTypes = []EventType{"*"} // wildcard
	}

	for _, eventType := range eventTypes {
		b.subscribers[eventType] = append(b.subscribers[eventType], ch)
	}

	return ch
}

// Unsubscribe removes a subscription
func (b *Broker) Unsubscribe(ch <-chan Event, eventTypes ...EventType) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If no specific types provided, unsubscribe from all
	if len(eventTypes) == 0 {
		for eventType := range b.subscribers {
			b.removeChannel(eventType, ch)
		}
		return
	}

	for _, eventType := range eventTypes {
		b.removeChannel(eventType, ch)
	}
}

// Publish sends an event to all subscribers
func (b *Broker) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Send to specific subscribers
	if subscribers, ok := b.subscribers[event.Type]; ok {
		for _, ch := range subscribers {
			select {
			case ch <- event:
			default:
				// Channel full, skip this event
			}
		}
	}

	// Send to wildcard subscribers
	if wildcards, ok := b.subscribers["*"]; ok {
		for _, ch := range wildcards {
			select {
			case ch <- event:
			default:
				// Channel full, skip this event
			}
		}
	}
}

// PublishAsync sends an event asynchronously
func (b *Broker) PublishAsync(event Event) {
	go b.Publish(event)
}

// removeChannel removes a channel from a specific event type's subscribers
func (b *Broker) removeChannel(eventType EventType, target <-chan Event) {
	subscribers := b.subscribers[eventType]
	for i, ch := range subscribers {
		if ch == target {
			// Remove this channel
			b.subscribers[eventType] = append(subscribers[:i], subscribers[i+1:]...)
			// Close the channel
			close(ch)
			break
		}
	}

	// Clean up empty subscriber lists
	if len(b.subscribers[eventType]) == 0 {
		delete(b.subscribers, eventType)
	}
}

// Clear removes all subscriptions
func (b *Broker) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, subscribers := range b.subscribers {
		for _, ch := range subscribers {
			close(ch)
		}
	}

	b.subscribers = make(map[EventType][]chan Event)
}