package queue

import (
	"context"
	"sync"
)

// Deduplicator manages request cancellation and superseding.
// It tracks active requests and cancels them when newer ones arrive.
//
// This is critical for file watching: when files change rapidly,
// we cancel stale analyses and only run the latest.
//
// Used by: Manager (before enqueueing items)
// Purpose: Prevent wasted LLM cycles on outdated requests
type Deduplicator struct {
	// Track active requests by ID
	active map[string]context.CancelFunc
	mutex  sync.Mutex
}

// NewDeduplicator creates a new deduplicator.
func NewDeduplicator() *Deduplicator {
	return &Deduplicator{
		active: make(map[string]context.CancelFunc),
	}
}

// Register tracks a new active request.
// Called when a request starts processing.
func (d *Deduplicator) Register(id string, cancel context.CancelFunc) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	d.active[id] = cancel
}

// Unregister removes a completed request.
// Called when a request finishes (success or error).
func (d *Deduplicator) Unregister(id string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	delete(d.active, id)
}

// Cancel aborts a specific request if active.
// Returns true if the request was found and canceled.
func (d *Deduplicator) Cancel(id string) bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if cancel, exists := d.active[id]; exists {
		cancel()
		delete(d.active, id)
		return true
	}
	return false
}

// CancelMany aborts multiple requests.
// Used when a new request supersedes several old ones.
// Returns the number of requests actually canceled.
func (d *Deduplicator) CancelMany(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	canceled := 0
	for _, id := range ids {
		if cancel, exists := d.active[id]; exists {
			cancel()
			delete(d.active, id)
			canceled++
		}
	}
	return canceled
}

// CancelByType cancels all active requests of a given type.
// Useful for canceling all "startup_scan" when a new one starts.
// Returns the number of requests canceled.
func (d *Deduplicator) CancelByType(requestType string, getCurrentType func(string) string) int {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	canceled := 0
	for id, cancel := range d.active {
		if getCurrentType(id) == requestType {
			cancel()
			delete(d.active, id)
			canceled++
		}
	}
	return canceled
}

// ActiveCount returns the number of active requests.
// Useful for monitoring and debugging.
func (d *Deduplicator) ActiveCount() int {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	return len(d.active)
}

// ActiveIDs returns a list of active request IDs.
// Useful for debugging and UI display.
func (d *Deduplicator) ActiveIDs() []string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	ids := make([]string, 0, len(d.active))
	for id := range d.active {
		ids = append(ids, id)
	}
	return ids
}