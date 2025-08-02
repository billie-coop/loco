// Package csync provides thread-safe concurrent data structures.
//
// This package implements generic, thread-safe versions of common Go data structures
// like maps and slices. All operations are protected by read-write mutexes to ensure
// safe concurrent access from multiple goroutines.
//
// The collections also implement JSON marshaling/unmarshaling for persistence
// and provide rich APIs with functional programming patterns.
//
// Example usage:
//
//	// Thread-safe map
//	sessions := csync.NewMap[string, *Session]()
//	sessions.Set("abc123", session)
//	if session, exists := sessions.Get("abc123"); exists {
//		// Use session safely
//	}
//
//	// Thread-safe slice
//	messages := csync.NewSlice[*Message]()
//	messages.Append(msg1, msg2)
//	messages.Range(func(i int, msg *Message) bool {
//		fmt.Printf("Message %d: %s\n", i, msg.Content)
//		return true // Continue iteration
//	})
//
// All operations are thread-safe and can be called concurrently from multiple
// goroutines without additional synchronization.
package csync