package csync

import (
	"encoding/json"
	"sync"
)

// Slice is a thread-safe slice implementation with generic types.
// It uses a RWMutex for concurrent read access and exclusive write access.
type Slice[T any] struct {
	data []T
	mu   sync.RWMutex
}

// NewSlice creates a new thread-safe slice
func NewSlice[T any]() *Slice[T] {
	return &Slice[T]{
		data: make([]T, 0),
	}
}

// NewSliceWithCapacity creates a new thread-safe slice with specified capacity
func NewSliceWithCapacity[T any](capacity int) *Slice[T] {
	return &Slice[T]{
		data: make([]T, 0, capacity),
	}
}

// NewSliceFrom creates a new thread-safe slice from existing slice
func NewSliceFrom[T any](slice []T) *Slice[T] {
	s := &Slice[T]{
		data: make([]T, len(slice)),
	}
	copy(s.data, slice)
	return s
}

// Append adds elements to the end of the slice
func (s *Slice[T]) Append(elements ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = append(s.data, elements...)
}

// Prepend adds elements to the beginning of the slice
func (s *Slice[T]) Prepend(elements ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = append(elements, s.data...)
}

// Get retrieves an element by index, returns the element and whether index is valid
func (s *Slice[T]) Get(index int) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var zero T
	if index < 0 || index >= len(s.data) {
		return zero, false
	}
	return s.data[index], true
}

// Set updates an element at the specified index
func (s *Slice[T]) Set(index int, value T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if index < 0 || index >= len(s.data) {
		return false
	}
	s.data[index] = value
	return true
}

// Len returns the length of the slice
func (s *Slice[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// Cap returns the capacity of the slice
func (s *Slice[T]) Cap() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cap(s.data)
}

// IsEmpty returns true if the slice is empty
func (s *Slice[T]) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data) == 0
}

// First returns the first element and whether the slice is not empty
func (s *Slice[T]) First() (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var zero T
	if len(s.data) == 0 {
		return zero, false
	}
	return s.data[0], true
}

// Last returns the last element and whether the slice is not empty
func (s *Slice[T]) Last() (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var zero T
	if len(s.data) == 0 {
		return zero, false
	}
	return s.data[len(s.data)-1], true
}

// Pop removes and returns the last element
func (s *Slice[T]) Pop() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var zero T
	if len(s.data) == 0 {
		return zero, false
	}
	
	index := len(s.data) - 1
	element := s.data[index]
	s.data = s.data[:index]
	return element, true
}

// RemoveAt removes an element at the specified index
func (s *Slice[T]) RemoveAt(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if index < 0 || index >= len(s.data) {
		return false
	}
	
	s.data = append(s.data[:index], s.data[index+1:]...)
	return true
}

// Insert inserts elements at the specified index
func (s *Slice[T]) Insert(index int, elements ...T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if index < 0 || index > len(s.data) {
		return false
	}
	
	// Make room for new elements
	s.data = append(s.data, make([]T, len(elements))...)
	// Shift existing elements
	copy(s.data[index+len(elements):], s.data[index:])
	// Insert new elements
	copy(s.data[index:], elements)
	
	return true
}

// Range iterates over all elements in the slice.
// The function f is called for each element with its index. If f returns false, iteration stops.
func (s *Slice[T]) Range(f func(index int, value T) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for i, value := range s.data {
		if !f(i, value) {
			break
		}
	}
}

// Filter creates a new slice containing elements that match the predicate
func (s *Slice[T]) Filter(predicate func(T) bool) *Slice[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := NewSlice[T]()
	for _, value := range s.data {
		if predicate(value) {
			result.data = append(result.data, value)
		}
	}
	return result
}

// Find returns the first element that matches the predicate
func (s *Slice[T]) Find(predicate func(T) bool) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var zero T
	for _, value := range s.data {
		if predicate(value) {
			return value, true
		}
	}
	return zero, false
}

// Clear removes all elements from the slice
func (s *Slice[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = s.data[:0]
}

// Clone creates a shallow copy of the slice
func (s *Slice[T]) Clone() *Slice[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	clone := NewSliceWithCapacity[T](len(s.data))
	clone.data = append(clone.data, s.data...)
	return clone
}

// ToSlice returns a copy of the underlying slice (not thread-safe to use directly)
func (s *Slice[T]) ToSlice() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]T, len(s.data))
	copy(result, s.data)
	return result
}

// MarshalJSON implements json.Marshaler interface
func (s *Slice[T]) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.data)
}

// UnmarshalJSON implements json.Unmarshaler interface
func (s *Slice[T]) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return json.Unmarshal(data, &s.data)
}