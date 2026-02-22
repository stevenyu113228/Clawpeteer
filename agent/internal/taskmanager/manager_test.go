package taskmanager

import (
	"testing"
)

func TestAddAndGet(t *testing.T) {
	mgr := New()
	mgr.Add("task-1", "echo hello", 1234)

	task, ok := mgr.Get("task-1")
	if !ok {
		t.Fatal("expected task to be found")
	}
	if task.ID != "task-1" {
		t.Errorf("ID = %q, want %q", task.ID, "task-1")
	}
	if task.Command != "echo hello" {
		t.Errorf("Command = %q, want %q", task.Command, "echo hello")
	}
	if task.PID != 1234 {
		t.Errorf("PID = %d, want %d", task.PID, 1234)
	}
	if task.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", task.Status, StatusRunning)
	}
	if task.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}
}

func TestGetNotFound(t *testing.T) {
	mgr := New()
	_, ok := mgr.Get("nonexistent")
	if ok {
		t.Error("expected task not to be found")
	}
}

func TestListMultipleTasks(t *testing.T) {
	mgr := New()
	mgr.Add("task-1", "echo one", 100)
	mgr.Add("task-2", "echo two", 200)
	mgr.Add("task-3", "echo three", 300)

	tasks := mgr.List()
	if len(tasks) != 3 {
		t.Errorf("List() returned %d tasks, want 3", len(tasks))
	}

	// Verify all tasks are present
	ids := make(map[string]bool)
	for _, task := range tasks {
		ids[task.ID] = true
	}
	for _, id := range []string{"task-1", "task-2", "task-3"} {
		if !ids[id] {
			t.Errorf("task %q not found in list", id)
		}
	}
}

func TestComplete(t *testing.T) {
	mgr := New()
	mgr.Add("task-1", "echo hello", 1234)

	mgr.Complete("task-1", 0)

	task, ok := mgr.Get("task-1")
	if !ok {
		t.Fatal("expected task to be found")
	}
	if task.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", task.Status, StatusCompleted)
	}
	if task.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", task.ExitCode)
	}
	if task.EndTime.IsZero() {
		t.Error("EndTime should not be zero after completion")
	}
}

func TestCompleteWithNonZeroExitCode(t *testing.T) {
	mgr := New()
	mgr.Add("task-1", "false", 1234)

	mgr.Complete("task-1", 1)

	task, _ := mgr.Get("task-1")
	if task.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", task.ExitCode)
	}
}

func TestSetStatus(t *testing.T) {
	mgr := New()
	mgr.Add("task-1", "sleep 100", 1234)

	mgr.SetStatus("task-1", StatusKilled)

	task, _ := mgr.Get("task-1")
	if task.Status != StatusKilled {
		t.Errorf("Status = %q, want %q", task.Status, StatusKilled)
	}
}

func TestRunningCount(t *testing.T) {
	mgr := New()
	mgr.Add("task-1", "cmd1", 100)
	mgr.Add("task-2", "cmd2", 200)
	mgr.Add("task-3", "cmd3", 300)

	if count := mgr.RunningCount(); count != 3 {
		t.Errorf("RunningCount() = %d, want 3", count)
	}

	mgr.Complete("task-1", 0)
	if count := mgr.RunningCount(); count != 2 {
		t.Errorf("RunningCount() = %d, want 2", count)
	}

	mgr.SetStatus("task-2", StatusKilled)
	if count := mgr.RunningCount(); count != 1 {
		t.Errorf("RunningCount() = %d, want 1", count)
	}
}

func TestRemoveCompleted(t *testing.T) {
	mgr := New()
	mgr.Add("task-1", "cmd1", 100)
	mgr.Add("task-2", "cmd2", 200)
	mgr.Add("task-3", "cmd3", 300)

	mgr.Complete("task-1", 0)
	mgr.SetStatus("task-3", StatusError)

	mgr.RemoveCompleted()

	tasks := mgr.List()
	if len(tasks) != 1 {
		t.Errorf("List() returned %d tasks after RemoveCompleted, want 1", len(tasks))
	}

	// Only running task should remain
	_, ok := mgr.Get("task-2")
	if !ok {
		t.Error("running task-2 should not have been removed")
	}

	_, ok = mgr.Get("task-1")
	if ok {
		t.Error("completed task-1 should have been removed")
	}

	_, ok = mgr.Get("task-3")
	if ok {
		t.Error("error task-3 should have been removed")
	}
}
