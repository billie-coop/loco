package watcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/billie-coop/loco/internal/files"
)

// ChangeType represents the type of file system change
type ChangeType int

const (
	ChangeModified ChangeType = iota
	ChangeCreated
	ChangeDeleted
	ChangeRenamed
)

func (c ChangeType) String() string {
	switch c {
	case ChangeModified:
		return "modified"
	case ChangeCreated:
		return "created"
	case ChangeDeleted:
		return "deleted"
	case ChangeRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// FileChangeEvent represents a file system change event
type FileChangeEvent struct {
	Paths []string
	Type  ChangeType
}

// FileWatcher monitors file system changes with debouncing.
// It collects rapid changes and triggers a single analysis after things settle.
//
// This is the core component for reactive analysis in Loco.
// When files change, it waits for a quiet period before triggering analysis.
//
// Used by: Main app (in sidecar mode), Agent (when generating code)
// Connects to: Queue manager (submits analysis requests)
type FileWatcher struct {
	// Configuration
	debounceDelay time.Duration
	ignorePaths   []string
	
	// Debouncing state
	timer       *time.Timer
	timerMu     sync.Mutex
	pendingPaths map[string]ChangeType
	
	// Event subscription
	subscribers []func(FileChangeEvent)
	subMu       sync.RWMutex
	
	// File system watcher
	fsWatcher *fsnotify.Watcher
	watchPath string
	
	// Legacy callback when changes are ready
	onChange func([]string)
	
	// Track last analysis ID for superseding
	lastAnalysisID string
	lastMu         sync.Mutex
	
	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWatcher creates a file watcher with the specified debounce delay.
// The onChange callback is called with changed paths after debouncing.
//
// Example:
//
//	watcher := NewWatcher(2*time.Second, func(paths []string) {
//	    fmt.Printf("Files changed: %v\n", paths)
//	    triggerAnalysis(paths)
//	})
func NewWatcher(debounceDelay time.Duration, onChange func([]string)) *FileWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &FileWatcher{
		debounceDelay: debounceDelay,
		ignorePaths:   defaultIgnorePaths(),
		pendingPaths:  make(map[string]ChangeType),
		onChange:      onChange,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Subscribe adds a callback for file change events
// The callback will be called with debounced file changes
func (w *FileWatcher) Subscribe(callback func(FileChangeEvent)) {
	w.subMu.Lock()
	w.subscribers = append(w.subscribers, callback)
	w.subMu.Unlock()
}

// StartWatching begins file system monitoring for the given path
// This integrates with fsnotify to watch the file system
func (w *FileWatcher) StartWatching(watchPath string) error {
	w.watchPath = watchPath
	
	// Create fsnotify watcher
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	w.fsWatcher = fsWatcher
	
	// Add the path to watch
	err = fsWatcher.Add(watchPath)
	if err != nil {
		fsWatcher.Close()
		return fmt.Errorf("failed to watch path %s: %w", watchPath, err)
	}
	
	// Start the event processing goroutine
	w.wg.Add(1)
	go w.processFileEvents()
	
	return nil
}

// processFileEvents handles fsnotify events and triggers debouncing
func (w *FileWatcher) processFileEvents() {
	defer w.wg.Done()
	
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return // Watcher closed
			}
			
			// Convert fsnotify event to our ChangeType and trigger debounced processing
			changeType := w.convertEventType(event.Op)
			w.fileChanged(event.Name, changeType)
			
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return // Watcher closed
			}
			// TODO: Could emit error events to subscribers if needed
			_ = err // For now, ignore errors silently
			
		case <-w.ctx.Done():
			return // Context cancelled
		}
	}
}

// convertEventType converts fsnotify operations to our ChangeType
func (w *FileWatcher) convertEventType(op fsnotify.Op) ChangeType {
	if op&fsnotify.Create == fsnotify.Create {
		return ChangeCreated
	}
	if op&fsnotify.Remove == fsnotify.Remove {
		return ChangeDeleted
	}
	if op&fsnotify.Rename == fsnotify.Rename {
		return ChangeRenamed
	}
	// Default to modified for Write and Chmod
	return ChangeModified
}

// fileChanged handles internal file change events with debouncing
func (w *FileWatcher) fileChanged(path string, changeType ChangeType) {
	// Check if should ignore using centralized rules
	if files.ShouldIgnore(path) {
		return
	}
	
	w.timerMu.Lock()
	defer w.timerMu.Unlock()
	
	// Add to pending with change type
	w.pendingPaths[path] = changeType
	
	// Reset timer
	if w.timer != nil {
		w.timer.Stop()
	}
	
	// Start new timer
	w.timer = time.AfterFunc(w.debounceDelay, w.processPending)
}

// FileChanged notifies the watcher of a file change (legacy API)
// Multiple rapid calls are debounced into a single onChange callback.
//
// This is the main entry point called by file system monitors.
func (w *FileWatcher) FileChanged(path string) {
	w.fileChanged(path, ChangeModified)
}

// FilesChanged notifies the watcher of multiple file changes.
// Useful for batch operations like git checkout.
func (w *FileWatcher) FilesChanged(paths []string) {
	w.timerMu.Lock()
	defer w.timerMu.Unlock()
	
	// Add all non-ignored paths
	added := false
	for _, path := range paths {
		if !w.shouldIgnore(path) {
			w.pendingPaths[path] = ChangeModified
			added = true
		}
	}
	
	if !added {
		return // All paths ignored
	}
	
	// Reset timer
	if w.timer != nil {
		w.timer.Stop()
	}
	
	// Start new timer
	w.timer = time.AfterFunc(w.debounceDelay, w.processPending)
}

// SetIgnorePaths updates the paths to ignore.
// Use this to filter out generated files, build output, etc.
func (w *FileWatcher) SetIgnorePaths(paths []string) {
	w.ignorePaths = paths
}

// SetLastAnalysisID tracks the last analysis for superseding.
// The next analysis will supersede this one.
func (w *FileWatcher) SetLastAnalysisID(id string) {
	w.lastMu.Lock()
	defer w.lastMu.Unlock()
	w.lastAnalysisID = id
}

// GetLastAnalysisID returns the ID to supersede.
func (w *FileWatcher) GetLastAnalysisID() string {
	w.lastMu.Lock()
	defer w.lastMu.Unlock()
	return w.lastAnalysisID
}

// Stop shuts down the watcher.
func (w *FileWatcher) Stop() {
	w.cancel()
	
	// Close fsnotify watcher
	if w.fsWatcher != nil {
		w.fsWatcher.Close()
	}
	
	// Wait for goroutines to finish
	w.wg.Wait()
	
	w.timerMu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timerMu.Unlock()
}

// processPending is called after debounce delay.
// It triggers both new event subscribers and legacy onChange callback.
func (w *FileWatcher) processPending() {
	w.timerMu.Lock()
	
	// Group paths by change type
	pathsByType := make(map[ChangeType][]string)
	allPaths := make([]string, 0, len(w.pendingPaths))
	
	for path, changeType := range w.pendingPaths {
		pathsByType[changeType] = append(pathsByType[changeType], path)
		allPaths = append(allPaths, path)
	}
	
	// Clear pending
	w.pendingPaths = make(map[string]ChangeType)
	w.timer = nil
	
	w.timerMu.Unlock()
	
	// Trigger new event subscribers (outside lock)
	if len(allPaths) > 0 {
		w.subMu.RLock()
		subscribers := make([]func(FileChangeEvent), len(w.subscribers))
		copy(subscribers, w.subscribers)
		w.subMu.RUnlock()
		
		// Send events grouped by type
		for changeType, paths := range pathsByType {
			event := FileChangeEvent{
				Paths: paths,
				Type:  changeType,
			}
			
			for _, subscriber := range subscribers {
				subscriber(event)
			}
		}
		
		// Trigger legacy callback for backward compatibility
		if w.onChange != nil {
			w.onChange(allPaths)
		}
	}
}

// shouldIgnore checks if a path should be ignored.
// Filters out common non-source files.
func (w *FileWatcher) shouldIgnore(path string) bool {
	// Use centralized file rules
	return files.ShouldIgnore(path)
}

// defaultIgnorePaths returns standard paths to ignore.
// These are directories we never want to watch.
func defaultIgnorePaths() []string {
	return []string{
		"node_modules",
		".git",
		"vendor",
		"build",
		"dist",
		"out",
		".next",
		"target",
		"__pycache__",
		".pytest_cache",
		".vscode",
		".idea",
	}
}

// DebounceMode represents different debouncing strategies.
type DebounceMode int

const (
	// DebounceNormal waits for quiet period (default)
	DebounceNormal DebounceMode = iota
	
	// DebounceAdaptive adjusts delay based on change frequency
	DebounceAdaptive
	
	// DebounceImmediate triggers immediately, then ignores for delay
	DebounceImmediate
)

// Config holds watcher configuration.
// Use this for more advanced setups.
type Config struct {
	DebounceDelay time.Duration
	IgnorePaths   []string
	Mode          DebounceMode
	MaxBatchSize  int // Max paths per onChange call
}

// NewWatcherWithConfig creates a watcher with custom configuration.
// Use this for more control over behavior.
func NewWatcherWithConfig(cfg Config, onChange func([]string)) *FileWatcher {
	if cfg.DebounceDelay == 0 {
		cfg.DebounceDelay = 2 * time.Second
	}
	if len(cfg.IgnorePaths) == 0 {
		cfg.IgnorePaths = defaultIgnorePaths()
	}
	
	watcher := NewWatcher(cfg.DebounceDelay, onChange)
	watcher.ignorePaths = cfg.IgnorePaths
	
	// Could implement different modes here
	// For now, just use normal debouncing
	
	return watcher
}

// WatchProject starts watching a project directory.
// This would integrate with fsnotify or similar.
// For now, it's a placeholder showing the interface.
func WatchProject(projectPath string, watcher *FileWatcher) error {
	// This would use fsnotify or similar to watch the directory
	// and call watcher.FileChanged() for each change.
	//
	// Example integration:
	//
	// watcher, _ := fsnotify.NewWatcher()
	// watcher.Add(projectPath)
	// for event := range watcher.Events {
	//     if event.Op&fsnotify.Write == fsnotify.Write {
	//         fileWatcher.FileChanged(event.Name)
	//     }
	// }
	
	return fmt.Errorf("not implemented: integrate with fsnotify")
}