package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"
)

// Result holds the output of a synchronous command execution.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// Process holds the state of an asynchronously spawned command.
type Process struct {
	PID    int
	Cmd    *exec.Cmd
	Stdout io.ReadCloser
	Stderr io.ReadCloser
	Done   chan Result
	cancel context.CancelFunc
}

// Kill terminates the spawned process.
func (p *Process) Kill() error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.Cmd != nil && p.Cmd.Process != nil {
		return p.Cmd.Process.Kill()
	}
	return nil
}

// shellArgs returns the shell and arguments for command execution
// based on the current operating system.
func shellArgs(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", []string{"/C", command}
	}
	return "bash", []string{"-c", command}
}

// ExecSync runs a command synchronously with a timeout and returns the result.
func ExecSync(command string, timeout time.Duration) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shell, args := shellArgs(command)
	cmd := exec.CommandContext(ctx, shell, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", timeout)
	}

	result := &Result{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("exec error: %w", err)
		}
	}

	return result, nil
}

// Spawn runs a command asynchronously and returns a Process with
// PID, stdout/stderr pipes, and a Done channel that receives the
// result when the command completes.
func Spawn(command string) (*Process, error) {
	ctx, cancel := context.WithCancel(context.Background())

	shell, args := shellArgs(command)
	cmd := exec.CommandContext(ctx, shell, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start command: %w", err)
	}

	proc := &Process{
		PID:    cmd.Process.Pid,
		Cmd:    cmd,
		Stdout: stdout,
		Stderr: stderr,
		Done:   make(chan Result, 1),
		cancel: cancel,
	}

	go func() {
		// Read all output before waiting
		stdoutBytes, _ := io.ReadAll(stdout)
		stderrBytes, _ := io.ReadAll(stderr)

		err := cmd.Wait()
		duration := time.Since(start)

		result := Result{
			Stdout:   string(stdoutBytes),
			Stderr:   string(stderrBytes),
			Duration: duration,
		}

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = -1
			}
		}

		proc.Done <- result
	}()

	return proc, nil
}
