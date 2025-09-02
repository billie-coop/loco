// Package watcher provides file system monitoring with intelligent debouncing.
//
// # Overview
//
// This package watches for file changes and triggers analysis, but with
// smart debouncing to avoid overwhelming the LLM with rapid changes.
// It's designed for both sidecar mode (watching external changes) and
// agent mode (coordinating with code generation).
//
// # Key Features
//
//   - Debounced change detection (configurable delay)
//   - Path filtering (ignore generated files, build output)
//   - Cancel-and-restart on new changes
//   - Integration with queue system
//
// # Architecture
//
// The watcher consists of:
//   - FileWatcher: Core watching logic with debouncing
//   - ChangeHandler: Callback interface for handling changes
//   - Filters: Composable path filtering
//
// # Usage in Loco
//
// Sidecar Mode:
//   - Watches project files for external changes
//   - Triggers startup scan on changes
//   - Longer debounce (2-3 seconds)
//
// Agent Mode:
//   - Coordinates with code generation
//   - Shorter debounce (1-2 seconds)
//   - Ignores own changes
//
// # Integration
//
// The watcher submits requests to the queue system:
//
//	watcher := NewWatcher(func(paths []string) {
//	    queueManager.Submit(ctx, runStartupScan,
//	        WithPriority(3),
//	        WithType("file_change"),
//	        WithSupersedes(previousScanID),
//	    )
//	})
//
package watcher