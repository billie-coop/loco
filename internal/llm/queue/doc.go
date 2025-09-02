// Package queue provides a robust, composable queuing system for LLM requests.
//
// # Overview
//
// This package solves the problem of managing multiple LLM requests to LM Studio
// when it can only handle limited concurrency. It provides:
//   - Priority-based scheduling (user requests > background tasks)
//   - Request deduplication (cancel stale analyses)
//   - Graceful cancellation (all requests are context-aware)
//   - Adaptive concurrency (adjust to model capacity)
//
// # Architecture
//
// The queue system consists of composable parts:
//
//   - QueueItem: Represents a single LLM request with priority and context
//   - Queue: Priority queue that holds and sorts items
//   - Processor: Worker that executes items from the queue
//   - Deduplicator: Cancels superseded requests
//   - Manager: Coordinates all components
//
// # Usage in Loco
//
// The queue is used throughout Loco to manage LLM requests:
//
//   - Startup scans: Progressive enhancement on each run
//   - File watching: Debounced analysis on file changes
//   - User commands: High-priority immediate execution
//   - Background analysis: Low-priority when idle
//
// # Integration Points
//
//   - tools/startup_scan.go: Submits startup scan requests
//   - tools/analyze.go: Submits analysis requests
//   - app/chat.go: Submits chat completion requests
//   - watcher/watcher.go: Triggers analysis on file changes
//
// # Example
//
//	manager := queue.NewManager()
//	
//	// Submit high-priority user request
//	id := manager.Submit(ctx, func(ctx context.Context) error {
//	    return runAnalysis(ctx)
//	}, queue.WithPriority(10))
//	
//	// Cancel if needed
//	manager.Cancel(id)
//
package queue