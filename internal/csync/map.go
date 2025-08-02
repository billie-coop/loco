package csync

import (
	"encoding/json"
	"sync"
)

// Map is a thread-safe map implementation with generic types.
// It uses a RWMutex for concurrent read access and exclusive write access.
type Map[K comparable, V any] struct {
	data map[K]V
	mu   sync.RWMutex
}

// NewMap creates a new thread-safe map
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		data: make(map[K]V),
	}
}

// Set stores a key-value pair in the map
func (m *Map[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

// Get retrieves a value by key, returns the value and whether it exists
func (m *Map[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.data[key]
	return value, exists
}

// Delete removes a key-value pair from the map
func (m *Map[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// Has checks if a key exists in the map
func (m *Map[K, V]) Has(key K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.data[key]
	return exists
}

// Len returns the number of key-value pairs in the map
func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// Keys returns a slice of all keys in the map
func (m *Map[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	keys := make([]K, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys
}

// Values returns a slice of all values in the map
func (m *Map[K, V]) Values() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	values := make([]V, 0, len(m.data))
	for _, value := range m.data {
		values = append(values, value)
	}
	return values
}

// Range iterates over all key-value pairs in the map.
// The function f is called for each pair. If f returns false, iteration stops.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for key, value := range m.data {
		if !f(key, value) {
			break
		}
	}
}

// Clear removes all key-value pairs from the map
func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[K]V)
}

// Clone creates a shallow copy of the map
func (m *Map[K, V]) Clone() *Map[K, V] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	clone := NewMap[K, V]()
	for key, value := range m.data {
		clone.data[key] = value
	}
	return clone
}

// ToMap returns a copy of the underlying map (not thread-safe to use directly)
func (m *Map[K, V]) ToMap() map[K]V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[K]V, len(m.data))
	for key, value := range m.data {
		result[key] = value
	}
	return result
}

// MarshalJSON implements json.Marshaler interface
func (m *Map[K, V]) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.Marshal(m.data)
}

// UnmarshalJSON implements json.Unmarshaler interface
func (m *Map[K, V]) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.data == nil {
		m.data = make(map[K]V)
	}
	
	return json.Unmarshal(data, &m.data)
}