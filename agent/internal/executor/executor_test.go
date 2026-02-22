package executor

import (
	"strings"
	"testing"
	"time"
)

func TestExecSyncSimpleCommand(t *testing.T) {
	result, err := ExecSync("echo hello", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	stdout := strings.TrimSpace(result.Stdout)
	if stdout != "hello" {
		t.Errorf("Stdout = %q, want %q", stdout, "hello")
	}
}

func TestExecSyncNonexistentCommand(t *testing.T) {
	result, err := ExecSync("nonexistent_command_xyz_12345", 5*time.Second)
	// Either an error is returned or exit code is non-zero
	if err == nil && result.ExitCode == 0 {
		t.Error("expected error or non-zero exit code for nonexistent command")
	}
}

func TestExecSyncTimeout(t *testing.T) {
	_, err := ExecSync("sleep 10", 1*time.Second)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, expected to contain 'timed out'", err.Error())
	}
}

func TestExecSyncStderr(t *testing.T) {
	result, err := ExecSync("echo error_msg >&2", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stderr := strings.TrimSpace(result.Stderr)
	if stderr != "error_msg" {
		t.Errorf("Stderr = %q, want %q", stderr, "error_msg")
	}
}

func TestSpawnSimpleCommand(t *testing.T) {
	proc, err := Spawn("echo spawned")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proc.PID <= 0 {
		t.Errorf("PID = %d, want > 0", proc.PID)
	}

	// Wait for completion
	select {
	case result := <-proc.Done:
		if result.ExitCode != 0 {
			t.Errorf("ExitCode = %d, want 0", result.ExitCode)
		}
		stdout := strings.TrimSpace(result.Stdout)
		if stdout != "spawned" {
			t.Errorf("Stdout = %q, want %q", stdout, "spawned")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for spawned process to complete")
	}
}

func TestSpawnKill(t *testing.T) {
	proc, err := Spawn("sleep 60")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proc.PID <= 0 {
		t.Errorf("PID = %d, want > 0", proc.PID)
	}

	// Kill the process
	if err := proc.Kill(); err != nil {
		t.Fatalf("Kill error: %v", err)
	}

	// Wait for Done signal
	select {
	case result := <-proc.Done:
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code after kill")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for killed process to report done")
	}
}
