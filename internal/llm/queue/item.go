package queue

import (
	"context"
	"time"
)

// QueueItem represents a single request in the LLM queue.
// It encapsulates everything needed to execute and manage an LLM request.
//
// Items are prioritized (higher priority = executed first) and can supersede
// other items (causing them to be canceled). Each item has its own context
// for cancellation.
//
// Used by: All LLM-calling code (analysis, chat, tools)
// Created by: Manager.Submit() with options
type QueueItem struct {
	// ID uniquely identifies this request for tracking and cancellation
	ID string
	
	// Priority determines execution order (higher = sooner)
	// 10 = user chat, 5 = user command, 3 = file change, 1 = background
	Priority int
	
	// Type categorizes the request for metrics and debugging
	// e.g., "startup_scan", "quick_analysis", "chat", "tool"
	Type string
	
	// Request is the actual function to execute with LLM
	// It receives a context that may be canceled if superseded
	Request func(context.Context) error
	
	// Context for this specific request (can be canceled)
	Context context.Context
	
	// Cancel function to abort this request
	Cancel context.CancelFunc
	
	// Supersedes contains IDs of requests this replaces
	// Those requests will be canceled when this is enqueued
	Supersedes []string
	
	// Created timestamp for age-based decisions
	Created time.Time
	
	// Metadata for debugging and logging
	Metadata map[string]interface{}
}

// Option configures a QueueItem when creating it.
// This pattern makes the API composable and extensible.
type Option func(*QueueItem)

// WithPriority sets the execution priority.
// Higher values execute first.
//
// Standard priorities:
//   - 10: User chat messages
//   - 5:  User commands (/scan, /analyze)
//   - 3:  File change triggered
//   - 1:  Background tasks
func WithPriority(p int) Option {
	return func(qi *QueueItem) {
		qi.Priority = p
	}
}

// WithType categorizes the request for debugging and metrics.
// Examples: "startup_scan", "quick_analysis", "chat", "tool"
func WithType(t string) Option {
	return func(qi *QueueItem) {
		qi.Type = t
	}
}

// WithSupersedes marks this request as replacing others.
// The specified requests will be canceled when this is enqueued.
// Use this for progressive enhancement and deduplication.
func WithSupersedes(ids ...string) Option {
	return func(qi *QueueItem) {
		qi.Supersedes = append(qi.Supersedes, ids...)
	}
}

// WithMetadata attaches arbitrary data for debugging.
// Useful for tracking request origin, file paths, etc.
func WithMetadata(key string, value interface{}) Option {
	return func(qi *QueueItem) {
		if qi.Metadata == nil {
			qi.Metadata = make(map[string]interface{})
		}
		qi.Metadata[key] = value
	}
}

// NewQueueItem creates a QueueItem with the given options.
// This is typically called by Manager.Submit(), not directly.
func NewQueueItem(id string, ctx context.Context, request func(context.Context) error, opts ...Option) *QueueItem {
	itemCtx, cancel := context.WithCancel(ctx)
	
	item := &QueueItem{
		ID:       id,
		Priority: 5, // Default middle priority
		Type:     "generic",
		Request:  request,
		Context:  itemCtx,
		Cancel:   cancel,
		Created:  time.Now(),
		Metadata: make(map[string]interface{}),
	}
	
	// Apply options
	for _, opt := range opts {
		opt(item)
	}
	
	return item
}