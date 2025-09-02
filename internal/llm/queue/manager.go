package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Manager coordinates all queue components.
// This is the main entry point for the queue system.
//
// It ties together:
//   - Queue (holds items)
//   - Processor (executes items)
//   - Deduplicator (cancels superseded items)
//
// Used by: The main app to manage all LLM requests
// Integrates with: LLM clients, file watcher, tools
type Manager struct {
	queue   *Queue
	proc    *Processor
	dedup   *Deduplicator
	
	// ID generation
	nextID  atomic.Uint64
	
	// Track items by ID for status queries
	items   map[string]*QueueItem
	itemsMu sync.RWMutex
	
	// Configuration
	maxWorkers int
	
	// Lifecycle
	started bool
	mutex   sync.Mutex
}

// NewManager creates a queue manager with default settings.
// maxWorkers controls LLM request parallelism (start with 1).
func NewManager(maxWorkers int) *Manager {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	
	queue := NewQueue()
	proc := NewProcessor(queue, maxWorkers)
	dedup := NewDeduplicator()
	
	m := &Manager{
		queue:      queue,
		proc:       proc,
		dedup:      dedup,
		items:      make(map[string]*QueueItem),
		maxWorkers: maxWorkers,
	}
	
	// Hook up callbacks
	proc.OnStart(m.onItemStart)
	proc.OnComplete(m.onItemComplete)
	
	return m
}

// Start begins processing queued items.
// Call this once during app initialization.
func (m *Manager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.started {
		return fmt.Errorf("manager already started")
	}
	
	m.proc.Start()
	m.started = true
	return nil
}

// Stop gracefully shuts down the queue system.
// Waits for in-flight requests to complete.
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if !m.started {
		return fmt.Errorf("manager not started")
	}
	
	m.proc.Stop()
	m.started = false
	return nil
}

// Submit adds a request to the queue.
// Returns a unique ID that can be used to cancel the request.
//
// Example:
//
//	id := manager.Submit(ctx, func(ctx context.Context) error {
//	    return llmClient.Complete(ctx, messages)
//	}, WithPriority(10), WithType("chat"))
func (m *Manager) Submit(ctx context.Context, request func(context.Context) error, opts ...Option) string {
	// Generate unique ID
	id := fmt.Sprintf("req_%d_%d", time.Now().Unix(), m.nextID.Add(1))
	
	// Create item
	item := NewQueueItem(id, ctx, request, opts...)
	
	// Handle superseding
	if len(item.Supersedes) > 0 {
		// Cancel superseded items
		m.dedup.CancelMany(item.Supersedes)
		
		// Remove from queue if not started
		for _, oldID := range item.Supersedes {
			m.queue.Remove(oldID)
			m.removeItem(oldID)
		}
	}
	
	// Track item
	m.addItem(item)
	
	// Register for deduplication
	m.dedup.Register(id, item.Cancel)
	
	// Enqueue
	m.queue.Push(item)
	
	return id
}

// Cancel aborts a request by ID.
// Returns true if the request was found and canceled.
func (m *Manager) Cancel(id string) bool {
	// Try to cancel if running
	canceled := m.dedup.Cancel(id)
	
	// Try to remove from queue if pending
	removed := m.queue.Remove(id)
	
	// Clean up tracking
	if canceled || removed {
		m.removeItem(id)
		return true
	}
	
	return false
}

// CancelByType cancels all requests of a given type.
// Useful for canceling all analyses when files change.
func (m *Manager) CancelByType(requestType string) int {
	m.itemsMu.RLock()
	ids := make([]string, 0)
	for id, item := range m.items {
		if item.Type == requestType {
			ids = append(ids, id)
		}
	}
	m.itemsMu.RUnlock()
	
	canceled := 0
	for _, id := range ids {
		if m.Cancel(id) {
			canceled++
		}
	}
	return canceled
}

// Status returns current queue status for monitoring.
type Status struct {
	Pending   int
	Active    int
	Completed int
	Canceled  int
	AvgTime   time.Duration
	ErrorRate float64
}

// GetStatus returns current queue metrics.
// Use this for UI display and monitoring.
func (m *Manager) GetStatus() Status {
	avgTime, errorRate := m.proc.GetMetrics()
	
	return Status{
		Pending:   m.queue.Len(),
		Active:    m.dedup.ActiveCount(),
		AvgTime:   avgTime,
		ErrorRate: errorRate,
	}
}

// SetMaxWorkers adjusts parallelism dynamically.
// Use this to adapt to model capacity.
func (m *Manager) SetMaxWorkers(n int) {
	if n < 1 {
		n = 1
	}
	m.maxWorkers = n
	m.proc.SetMaxWorkers(n)
}

// AdaptConcurrency automatically adjusts workers based on performance.
// Call this periodically or after each request.
func (m *Manager) AdaptConcurrency() {
	avgTime, _ := m.proc.GetMetrics()
	
	// Simple heuristic: reduce parallelism if responses are slow
	if avgTime > 15*time.Second && m.maxWorkers > 1 {
		m.SetMaxWorkers(m.maxWorkers - 1)
	} else if avgTime < 5*time.Second && m.maxWorkers < 3 {
		m.SetMaxWorkers(m.maxWorkers + 1)
	}
}

// Internal callbacks

func (m *Manager) onItemStart(item *QueueItem) {
	// Could emit events here for UI updates
	fmt.Printf("[Queue] Starting %s (%s) priority=%d\n", item.ID, item.Type, item.Priority)
}

func (m *Manager) onItemComplete(item *QueueItem, err error, duration time.Duration) {
	// Unregister from deduplicator
	m.dedup.Unregister(item.ID)
	
	// Remove from tracking
	m.removeItem(item.ID)
	
	// Adapt concurrency based on performance
	m.AdaptConcurrency()
	
	// Log completion
	if err != nil {
		fmt.Printf("[Queue] Failed %s (%s) after %v: %v\n", item.ID, item.Type, duration, err)
	} else {
		fmt.Printf("[Queue] Completed %s (%s) in %v\n", item.ID, item.Type, duration)
	}
}

func (m *Manager) addItem(item *QueueItem) {
	m.itemsMu.Lock()
	defer m.itemsMu.Unlock()
	m.items[item.ID] = item
}

func (m *Manager) removeItem(id string) {
	m.itemsMu.Lock()
	defer m.itemsMu.Unlock()
	delete(m.items, id)
}