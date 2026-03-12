package pocketflow

import "sync"

type eventBus struct {
	mu      sync.Mutex
	waiters map[string][]chan struct{}
}

func newEventBus() *eventBus {
	return &eventBus{
		waiters: make(map[string][]chan struct{}),
	}
}

// Wait registers a waiter for the given key and returns a channel that will be signaled.
func (eb *eventBus) Wait(key string) chan struct{} {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan struct{}, 1)
	eb.waiters[key] = append(eb.waiters[key], ch)
	return ch
}

// Notify signals all waiters for the given key and removes them.
func (eb *eventBus) Notify(key string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for _, ch := range eb.waiters[key] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	delete(eb.waiters, key)
}

// Remove unregisters a specific waiter channel for the given key.
func (eb *eventBus) Remove(key string, ch chan struct{}) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.removeLocked(key, ch)
}

// Swap atomically replaces a waiter channel with a new one, preventing
// a gap where a Notify could be missed between Remove and Wait.
func (eb *eventBus) Swap(key string, old chan struct{}) chan struct{} {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.removeLocked(key, old)
	ch := make(chan struct{}, 1)
	eb.waiters[key] = append(eb.waiters[key], ch)
	return ch
}

func (eb *eventBus) removeLocked(key string, ch chan struct{}) {
	waiters := eb.waiters[key]
	for i, w := range waiters {
		if w == ch {
			eb.waiters[key] = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(eb.waiters[key]) == 0 {
		delete(eb.waiters, key)
	}
}
