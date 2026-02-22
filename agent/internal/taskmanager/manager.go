package taskmanager

import (
	"sync"
	"time"
)

// Status constants for task lifecycle.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusKilled    = "killed"
	StatusError     = "error"
)

// Task represents a tracked command execution.
type Task struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	PID       int       `json:"pid"`
	Status    string    `json:"status"`
	ExitCode  int       `json:"exitCode"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime,omitempty"`
}

// Manager provides thread-safe task tracking.
type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// New creates a new task Manager.
func New() *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
	}
}

// Add registers a new task with status "running".
func (m *Manager) Add(id, command string, pid int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[id] = &Task{
		ID:        id,
		Command:   command,
		PID:       pid,
		Status:    StatusRunning,
		StartTime: time.Now(),
	}
}

// Get retrieves a task by ID. Returns the task and true if found,
// or nil and false if not found.
func (m *Manager) Get(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, false
	}

	// Return a copy to avoid data races on the Task fields
	copy := *task
	return &copy, true
}

// List returns a slice of all tracked tasks.
func (m *Manager) List() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		copy := *task
		result = append(result, &copy)
	}
	return result
}

// Complete marks a task as completed with the given exit code
// and records the end time.
func (m *Manager) Complete(id string, exitCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return
	}

	task.Status = StatusCompleted
	task.ExitCode = exitCode
	task.EndTime = time.Now()
}

// SetStatus updates the status of a task.
func (m *Manager) SetStatus(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return
	}

	task.Status = status
}

// RunningCount returns the number of tasks with status "running".
func (m *Manager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, task := range m.tasks {
		if task.Status == StatusRunning {
			count++
		}
	}
	return count
}

// RemoveCompleted removes all tasks that are not in "running" status.
func (m *Manager) RemoveCompleted() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, task := range m.tasks {
		if task.Status != StatusRunning {
			delete(m.tasks, id)
		}
	}
}
