package orchestrator

import (
	"fmt"
	"sync"
	"time"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

// Task represents a unit of work for the orchestrator
type Task struct {
	ID          string
	Type        TaskType
	Description string
	Status      TaskStatus
	ModelSize   ModelSize // Which size model is handling this
	ModelID     string    // Actual model working on it
	Input       string
	Output      string
	Error       error
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Dependencies []string // IDs of tasks that must complete first
}

// TaskList manages a queue of tasks
type TaskList struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	order    []string // Ordered list of task IDs
}

// NewTaskList creates a new task list
func NewTaskList() *TaskList {
	return &TaskList{
		tasks: make(map[string]*Task),
		order: make([]string, 0),
	}
}

// Add creates and adds a new task
func (tl *TaskList) Add(taskType TaskType, description, input string) *Task {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	
	task := &Task{
		ID:          fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Type:        taskType,
		Description: description,
		Status:      StatusPending,
		Input:       input,
		CreatedAt:   time.Now(),
	}
	
	tl.tasks[task.ID] = task
	tl.order = append(tl.order, task.ID)
	
	return task
}

// AddWithDependencies creates a task that depends on others
func (tl *TaskList) AddWithDependencies(taskType TaskType, description, input string, deps []string) *Task {
	task := tl.Add(taskType, description, input)
	
	tl.mu.Lock()
	task.Dependencies = deps
	tl.mu.Unlock()
	
	return task
}

// GetNext returns the next pending task that has no incomplete dependencies
func (tl *TaskList) GetNext() *Task {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	
	for _, id := range tl.order {
		task := tl.tasks[id]
		if task.Status != StatusPending {
			continue
		}
		
		// Check dependencies
		canRun := true
		for _, depID := range task.Dependencies {
			if dep, exists := tl.tasks[depID]; exists {
				if dep.Status != StatusCompleted {
					canRun = false
					break
				}
			}
		}
		
		if canRun {
			return task
		}
	}
	
	return nil
}

// UpdateStatus updates a task's status
func (tl *TaskList) UpdateStatus(taskID string, status TaskStatus) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	
	task, exists := tl.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}
	
	task.Status = status
	now := time.Now()
	
	switch status {
	case StatusInProgress:
		task.StartedAt = &now
	case StatusCompleted, StatusFailed:
		task.CompletedAt = &now
	}
	
	return nil
}

// SetOutput sets the output for a task
func (tl *TaskList) SetOutput(taskID, output string) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	
	task, exists := tl.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}
	
	task.Output = output
	return nil
}

// SetError sets an error for a task
func (tl *TaskList) SetError(taskID string, err error) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	
	task, exists := tl.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}
	
	task.Error = err
	task.Status = StatusFailed
	return nil
}

// GetAll returns all tasks in order
func (tl *TaskList) GetAll() []*Task {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	
	tasks := make([]*Task, 0, len(tl.order))
	for _, id := range tl.order {
		tasks = append(tasks, tl.tasks[id])
	}
	
	return tasks
}

// GetByStatus returns tasks with a specific status
func (tl *TaskList) GetByStatus(status TaskStatus) []*Task {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	
	var tasks []*Task
	for _, task := range tl.tasks {
		if task.Status == status {
			tasks = append(tasks, task)
		}
	}
	
	return tasks
}

// Clear removes all completed or failed tasks
func (tl *TaskList) Clear() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	
	newOrder := make([]string, 0)
	for _, id := range tl.order {
		task := tl.tasks[id]
		if task.Status == StatusPending || task.Status == StatusInProgress {
			newOrder = append(newOrder, id)
		} else {
			delete(tl.tasks, id)
		}
	}
	
	tl.order = newOrder
}

// Summary returns a summary of the task list
func (tl *TaskList) Summary() string {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	
	pending := 0
	inProgress := 0
	completed := 0
	failed := 0
	
	for _, task := range tl.tasks {
		switch task.Status {
		case StatusPending:
			pending++
		case StatusInProgress:
			inProgress++
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		}
	}
	
	return fmt.Sprintf("Tasks: %d pending, %d in progress, %d completed, %d failed",
		pending, inProgress, completed, failed)
}