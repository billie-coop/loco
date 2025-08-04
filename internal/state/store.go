package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store is a simple persistent key-value store that keeps data in memory and on disk.
// It's like Zustand with persist middleware - simple, reliable, and always in sync.
type Store[T any] struct {
	mu       sync.RWMutex
	data     T
	filepath string
	defaults T
}

// NewStore creates a new persistent store.
func NewStore[T any](filepath string, defaults T) *Store[T] {
	s := &Store[T]{
		filepath: filepath,
		defaults: defaults,
		data:     defaults,
	}
	
	// Load from disk if exists
	s.load()
	
	return s
}

// Get returns the current state.
func (s *Store[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Set updates the state and persists to disk.
func (s *Store[T]) Set(data T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.data = data
	return s.save()
}

// Update applies a function to modify the state and persists to disk.
func (s *Store[T]) Update(fn func(T) T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.data = fn(s.data)
	return s.save()
}

// load reads from disk if file exists.
func (s *Store[T]) load() {
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		// File doesn't exist or can't read - use defaults
		s.data = s.defaults
		return
	}
	
	if err := json.Unmarshal(data, &s.data); err != nil {
		// Corrupt file - use defaults
		s.data = s.defaults
	}
}

// save writes to disk.
func (s *Store[T]) save() error {
	// Ensure directory exists
	dir := filepath.Dir(s.filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Marshal to JSON
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	
	// Write atomically (write to temp file, then rename)
	tempFile := s.filepath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	
	// Rename to actual file (atomic on most systems)
	if err := os.Rename(tempFile, s.filepath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}
	
	return nil
}

// Clear resets to defaults and removes the file.
func (s *Store[T]) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.data = s.defaults
	return os.Remove(s.filepath)
}