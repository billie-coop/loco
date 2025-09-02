package queue

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Processor pulls items from a queue and executes them.
// It manages concurrency (how many requests run in parallel) and
// tracks metrics for adaptive behavior.
//
// The processor is the "worker" that actually runs LLM requests.
// It respects the maxWorkers limit to avoid overwhelming LM Studio.
//
// Used by: Manager (starts/stops it)
// Connects to: Queue (pulls items), LLM client (executes requests)
type Processor struct {
	queue      *Queue
	maxWorkers int
	semaphore  chan struct{}
	
	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Metrics for adaptation
	metrics struct {
		sync.Mutex
		totalProcessed   int
		totalErrors      int
		avgResponseTime  time.Duration
		lastResponseTime time.Duration
	}
	
	// Callbacks for monitoring
	onStart    func(item *QueueItem)
	onComplete func(item *QueueItem, err error, duration time.Duration)
}

// NewProcessor creates a processor with the specified concurrency limit.
// maxWorkers controls how many LLM requests can run in parallel.
// Start with 1 for safety, increase based on model size.
func NewProcessor(queue *Queue, maxWorkers int) *Processor {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Processor{
		queue:      queue,
		maxWorkers: maxWorkers,
		semaphore:  make(chan struct{}, maxWorkers),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins processing items from the queue.
// Runs in background goroutine until Stop is called.
func (p *Processor) Start() {
	p.wg.Add(1)
	go p.run()
}

// Stop gracefully shuts down the processor.
// Waits for in-flight requests to complete.
func (p *Processor) Stop() {
	p.cancel()
	p.wg.Wait()
}

// SetMaxWorkers adjusts concurrency limit.
// Use this to adapt to model capacity dynamically.
func (p *Processor) SetMaxWorkers(n int) {
	if n < 1 {
		n = 1
	}
	// Create new semaphore with new size
	// (simple approach, could be more sophisticated)
	p.semaphore = make(chan struct{}, n)
	p.maxWorkers = n
}

// OnStart sets callback for when item starts processing.
// Useful for UI updates and debugging.
func (p *Processor) OnStart(fn func(*QueueItem)) {
	p.onStart = fn
}

// OnComplete sets callback for when item finishes.
// Useful for metrics, logging, and UI updates.
func (p *Processor) OnComplete(fn func(*QueueItem, error, time.Duration)) {
	p.onComplete = fn
}

// GetMetrics returns current performance metrics.
// Use this to monitor and adapt behavior.
func (p *Processor) GetMetrics() (avgTime time.Duration, errorRate float64) {
	p.metrics.Lock()
	defer p.metrics.Unlock()
	
	if p.metrics.totalProcessed > 0 {
		errorRate = float64(p.metrics.totalErrors) / float64(p.metrics.totalProcessed)
	}
	return p.metrics.avgResponseTime, errorRate
}

// run is the main processor loop.
// Pulls items from queue and executes them with concurrency control.
func (p *Processor) run() {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		
		// Get next item (blocks if queue empty)
		item := p.queue.Pop()
		if item == nil {
			continue
		}
		
		// Check if already canceled
		select {
		case <-item.Context.Done():
			continue // Skip canceled items
		default:
		}
		
		// Acquire semaphore (wait for worker slot)
		select {
		case p.semaphore <- struct{}{}:
			// Got a slot, process item
			p.wg.Add(1)
			go p.process(item)
		case <-p.ctx.Done():
			return
		}
	}
}

// process executes a single queue item.
// Runs in its own goroutine with timeout and metrics tracking.
func (p *Processor) process(item *QueueItem) {
	defer p.wg.Done()
	defer func() { <-p.semaphore }() // Release worker slot
	
	// Notify start
	if p.onStart != nil {
		p.onStart(item)
	}
	
	// Track timing
	start := time.Now()
	
	// Execute with timeout (2 minutes default, could be configurable)
	ctx, cancel := context.WithTimeout(item.Context, 2*time.Minute)
	defer cancel()
	
	// Run the actual request
	err := item.Request(ctx)
	
	// Track metrics
	duration := time.Since(start)
	p.updateMetrics(err, duration)
	
	// Notify completion
	if p.onComplete != nil {
		p.onComplete(item, err, duration)
	}
	
	// Log errors for debugging
	if err != nil {
		fmt.Printf("Queue item %s (%s) failed: %v\n", item.ID, item.Type, err)
	}
}

// updateMetrics records performance data for adaptation.
func (p *Processor) updateMetrics(err error, duration time.Duration) {
	p.metrics.Lock()
	defer p.metrics.Unlock()
	
	p.metrics.totalProcessed++
	if err != nil {
		p.metrics.totalErrors++
	}
	
	p.metrics.lastResponseTime = duration
	
	// Simple moving average (could use exponential for recency bias)
	if p.metrics.avgResponseTime == 0 {
		p.metrics.avgResponseTime = duration
	} else {
		// Weight recent measurements more
		p.metrics.avgResponseTime = (p.metrics.avgResponseTime*4 + duration) / 5
	}
}