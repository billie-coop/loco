package watcher

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

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
	pendingPaths map[string]struct{}
	
	// Callback when changes are ready
	onChange func([]string)
	
	// Track last analysis ID for superseding
	lastAnalysisID string
	lastMu         sync.Mutex
	
	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
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
		pendingPaths:  make(map[string]struct{}),
		onChange:      onChange,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// FileChanged notifies the watcher of a file change.
// Multiple rapid calls are debounced into a single onChange callback.
//
// This is the main entry point called by file system monitors.
func (w *FileWatcher) FileChanged(path string) {
	// Check if should ignore
	if w.shouldIgnore(path) {
		return
	}
	
	w.timerMu.Lock()
	defer w.timerMu.Unlock()
	
	// Add to pending
	w.pendingPaths[path] = struct{}{}
	
	// Reset timer
	if w.timer != nil {
		w.timer.Stop()
	}
	
	// Start new timer
	w.timer = time.AfterFunc(w.debounceDelay, w.processPending)
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
			w.pendingPaths[path] = struct{}{}
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
	
	w.timerMu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timerMu.Unlock()
}

// processPending is called after debounce delay.
// It triggers the onChange callback with accumulated paths.
func (w *FileWatcher) processPending() {
	w.timerMu.Lock()
	
	// Collect paths
	paths := make([]string, 0, len(w.pendingPaths))
	for path := range w.pendingPaths {
		paths = append(paths, path)
	}
	
	// Clear pending
	w.pendingPaths = make(map[string]struct{})
	w.timer = nil
	
	w.timerMu.Unlock()
	
	// Trigger callback (outside lock)
	if len(paths) > 0 && w.onChange != nil {
		w.onChange(paths)
	}
}

// shouldIgnore checks if a path should be ignored.
// Filters out common non-source files.
func (w *FileWatcher) shouldIgnore(path string) bool {
	// Check absolute path components
	for _, ignore := range w.ignorePaths {
		if strings.Contains(path, ignore) {
			return true
		}
	}
	
	// Check filename patterns
	base := filepath.Base(path)
	
	// Ignore hidden files
	if strings.HasPrefix(base, ".") {
		return true
	}
	
	// Ignore common generated files
	ext := filepath.Ext(base)
	switch ext {
	case ".log", ".tmp", ".swp", ".swo", ".DS_Store":
		return true
	}
	
	return false
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