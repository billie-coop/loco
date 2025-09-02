package queue

import (
	"container/heap"
	"sync"
)

// Queue is a thread-safe priority queue for LLM requests.
// Items with higher priority are dequeued first.
// If priorities are equal, older items go first (FIFO within priority).
//
// This is the core data structure that holds pending requests.
// The Processor pulls items from here to execute them.
//
// Used by: Manager (adds items), Processor (removes items)
// Thread-safe: Yes (all operations lock)
type Queue struct {
	items priorityQueue
	mutex sync.Mutex
	cond  *sync.Cond
}

// NewQueue creates an empty priority queue.
func NewQueue() *Queue {
	q := &Queue{
		items: make(priorityQueue, 0),
	}
	q.cond = sync.NewCond(&q.mutex)
	heap.Init(&q.items)
	return q
}

// Push adds an item to the queue in priority order.
// Higher priority items will be dequeued first.
// Called by Manager when submitting new requests.
func (q *Queue) Push(item *QueueItem) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	heap.Push(&q.items, item)
	q.cond.Signal() // Wake up waiting processor
}

// Pop removes and returns the highest priority item.
// Blocks if queue is empty (waits for Push).
// Called by Processor to get next item to execute.
func (q *Queue) Pop() *QueueItem {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	// Wait for items
	for q.items.Len() == 0 {
		q.cond.Wait()
	}
	
	return heap.Pop(&q.items).(*QueueItem)
}

// TryPop is like Pop but returns nil immediately if queue is empty.
// Useful for non-blocking checks.
func (q *Queue) TryPop() *QueueItem {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	if q.items.Len() == 0 {
		return nil
	}
	
	return heap.Pop(&q.items).(*QueueItem)
}

// Remove removes an item by ID.
// Returns true if found and removed.
// Used when canceling requests that haven't started yet.
func (q *Queue) Remove(id string) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	for i, item := range q.items {
		if item.ID == id {
			heap.Remove(&q.items, i)
			return true
		}
	}
	return false
}

// Len returns the number of items in queue.
// Useful for monitoring and debugging.
func (q *Queue) Len() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.items.Len()
}

// Items returns a snapshot of all items (for debugging).
// The returned slice is a copy and safe to iterate.
func (q *Queue) Items() []*QueueItem {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	result := make([]*QueueItem, len(q.items))
	copy(result, q.items)
	return result
}

// priorityQueue implements heap.Interface for priority ordering.
// Higher priority items come first. Within same priority, older items come first.
type priorityQueue []*QueueItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// Higher priority first
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// Same priority: older first (FIFO)
	return pq[i].Created.Before(pq[j].Created)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*QueueItem))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}