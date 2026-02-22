package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stevenmeow/clawpeteer-agent/internal/executor"
	"github.com/stevenmeow/clawpeteer-agent/internal/security"
	"github.com/stevenmeow/clawpeteer-agent/internal/taskmanager"
)

// Command represents an incoming command message from the server.
type Command struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Command    string `json:"command"`
	Timeout    int    `json:"timeout"`
	Background bool   `json:"background"`
	Stream     bool   `json:"stream"`
	Timestamp  int64  `json:"timestamp"`
	// File download fields
	SourcePath string `json:"sourcePath,omitempty"`
	TransferID string `json:"transferId,omitempty"`
}

// ResultMessage represents an outgoing result message.
type ResultMessage struct {
	TaskID   string `json:"taskId"`
	Status   string `json:"status"`
	PID      int    `json:"pid,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Duration int64  `json:"duration,omitempty"`
	Error    string `json:"error,omitempty"`
	Signal   string `json:"signal,omitempty"`
	Timestamp int64 `json:"timestamp"`
}

// StreamMessage represents an outgoing stream message (QoS 0).
type StreamMessage struct {
	Type      string `json:"type"`
	Data      string `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

// Heartbeat represents the periodic heartbeat message.
type Heartbeat struct {
	Status       string `json:"status"`
	Platform     string `json:"platform"`
	Arch         string `json:"arch"`
	Hostname     string `json:"hostname"`
	Version      string `json:"version"`
	Uptime       int64  `json:"uptime"`
	RunningTasks int    `json:"runningTasks"`
	Timestamp    int64  `json:"timestamp"`
}

// ControlCommand represents an incoming control message.
type ControlCommand struct {
	Action string `json:"action"`
	Signal string `json:"signal,omitempty"`
}

// Handler orchestrates command execution, streaming, and heartbeat.
type Handler struct {
	agentID   string
	client    mqtt.Client
	tasks     *taskmanager.Manager
	security  *security.Validator
	processes map[string]*executor.Process
	mu        sync.Mutex
	startTime time.Time
	stopHB    chan struct{}
}

// New creates a new Handler.
func New(agentID string, client mqtt.Client, tasks *taskmanager.Manager, sec *security.Validator) *Handler {
	return &Handler{
		agentID:   agentID,
		client:    client,
		tasks:     tasks,
		security:  sec,
		processes: make(map[string]*executor.Process),
		startTime: time.Now(),
		stopHB:    make(chan struct{}),
	}
}

// Subscribe subscribes to the command and control topics.
func (h *Handler) Subscribe() {
	commandTopic := fmt.Sprintf("agents/%s/commands", h.agentID)
	controlTopic := fmt.Sprintf("agents/%s/control/+", h.agentID)

	h.client.Subscribe(commandTopic, 1, h.handleCommand)
	h.client.Subscribe(controlTopic, 1, h.handleControl)

	log.Printf("Subscribed to %s and %s", commandTopic, controlTopic)

	h.publishRegistry()
}

// handleCommand dispatches incoming command messages.
func (h *Handler) handleCommand(client mqtt.Client, msg mqtt.Message) {
	var cmd Command
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		log.Printf("Failed to parse command: %v", err)
		return
	}

	log.Printf("Received command id=%s type=%s", cmd.ID, cmd.Type)

	switch cmd.Type {
	case "execute":
		go h.executeCommand(cmd)
	case "file_download":
		go h.handleDownload(cmd)
	default:
		log.Printf("Unknown command type: %s", cmd.Type)
		h.publishResult(ResultMessage{
			TaskID:    cmd.ID,
			Status:    "error",
			Error:     fmt.Sprintf("unknown command type: %s", cmd.Type),
			Timestamp: time.Now().UnixMilli(),
		})
	}
}

// executeCommand validates and executes a command, publishing results.
func (h *Handler) executeCommand(cmd Command) {
	// Security check
	if err := h.security.ValidateCommand(cmd.Command); err != nil {
		h.publishResult(ResultMessage{
			TaskID:    cmd.ID,
			Status:    "error",
			Error:     fmt.Sprintf("security: %v", err),
			Timestamp: time.Now().UnixMilli(),
		})
		return
	}

	timeout := time.Duration(cmd.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// Background or streaming commands use Spawn
	if cmd.Background || cmd.Stream {
		proc, err := executor.Spawn(cmd.Command)
		if err != nil {
			h.publishResult(ResultMessage{
				TaskID:    cmd.ID,
				Status:    "error",
				Error:     fmt.Sprintf("spawn: %v", err),
				Timestamp: time.Now().UnixMilli(),
			})
			return
		}

		h.tasks.Add(cmd.ID, cmd.Command, proc.PID)

		h.mu.Lock()
		h.processes[cmd.ID] = proc
		h.mu.Unlock()

		// Publish started status
		h.publishResult(ResultMessage{
			TaskID:    cmd.ID,
			Status:    "started",
			PID:       proc.PID,
			Timestamp: time.Now().UnixMilli(),
		})

		// Stream output if requested
		if cmd.Stream {
			go h.streamOutput(cmd.ID, proc)
		}

		// Wait for completion in background
		go func() {
			result := <-proc.Done

			h.tasks.Complete(cmd.ID, result.ExitCode)

			h.mu.Lock()
			delete(h.processes, cmd.ID)
			h.mu.Unlock()

			h.publishResult(ResultMessage{
				TaskID:    cmd.ID,
				Status:    "completed",
				PID:       proc.PID,
				ExitCode:  result.ExitCode,
				Stdout:    result.Stdout,
				Stderr:    result.Stderr,
				Duration:  result.Duration.Milliseconds(),
				Timestamp: time.Now().UnixMilli(),
			})
		}()
		return
	}

	// Synchronous execution
	h.tasks.Add(cmd.ID, cmd.Command, 0)

	h.publishResult(ResultMessage{
		TaskID:    cmd.ID,
		Status:    "started",
		Timestamp: time.Now().UnixMilli(),
	})

	result, err := executor.ExecSync(cmd.Command, timeout)
	if err != nil {
		h.tasks.SetStatus(cmd.ID, taskmanager.StatusError)
		h.publishResult(ResultMessage{
			TaskID:    cmd.ID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now().UnixMilli(),
		})
		return
	}

	h.tasks.Complete(cmd.ID, result.ExitCode)

	h.publishResult(ResultMessage{
		TaskID:    cmd.ID,
		Status:    "completed",
		ExitCode:  result.ExitCode,
		Stdout:    result.Stdout,
		Stderr:    result.Stderr,
		Duration:  result.Duration.Milliseconds(),
		Timestamp: time.Now().UnixMilli(),
	})
}

// streamOutput reads stdout and stderr pipes from a spawned process
// and publishes each line to the stream topic at QoS 0.
func (h *Handler) streamOutput(taskID string, proc *executor.Process) {
	streamTopic := fmt.Sprintf("agents/%s/stream/%s", h.agentID, taskID)

	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(proc.Stdout)
		for scanner.Scan() {
			msg := StreamMessage{
				Type:      "stdout",
				Data:      scanner.Text(),
				Timestamp: time.Now().UnixMilli(),
			}
			data, _ := json.Marshal(msg)
			h.client.Publish(streamTopic, 0, false, data)
		}
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(proc.Stderr)
		for scanner.Scan() {
			msg := StreamMessage{
				Type:      "stderr",
				Data:      scanner.Text(),
				Timestamp: time.Now().UnixMilli(),
			}
			data, _ := json.Marshal(msg)
			h.client.Publish(streamTopic, 0, false, data)
		}
	}()

	wg.Wait()
}

// handleControl processes control commands (kill, query, list).
// The task ID is extracted from the topic: agents/{id}/control/{task-id}.
func (h *Handler) handleControl(client mqtt.Client, msg mqtt.Message) {
	// Extract task ID from topic
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) < 4 {
		log.Printf("Invalid control topic: %s", msg.Topic())
		return
	}
	taskID := parts[len(parts)-1]

	var ctrl ControlCommand
	if err := json.Unmarshal(msg.Payload(), &ctrl); err != nil {
		log.Printf("Failed to parse control command: %v", err)
		return
	}

	log.Printf("Control action=%s taskId=%s", ctrl.Action, taskID)

	switch ctrl.Action {
	case "kill":
		h.mu.Lock()
		proc, ok := h.processes[taskID]
		h.mu.Unlock()
		if !ok {
			h.publishResult(ResultMessage{
				TaskID:    taskID,
				Status:    "error",
				Error:     "process not found",
				Timestamp: time.Now().UnixMilli(),
			})
			return
		}
		if err := proc.Kill(); err != nil {
			log.Printf("Failed to kill process %s: %v", taskID, err)
		}
		h.tasks.SetStatus(taskID, taskmanager.StatusKilled)
		h.publishResult(ResultMessage{
			TaskID:    taskID,
			Status:    "killed",
			Signal:    ctrl.Signal,
			Timestamp: time.Now().UnixMilli(),
		})

	case "query":
		task, ok := h.tasks.Get(taskID)
		if !ok {
			h.publishResult(ResultMessage{
				TaskID:    taskID,
				Status:    "error",
				Error:     "task not found",
				Timestamp: time.Now().UnixMilli(),
			})
			return
		}
		h.publishResult(ResultMessage{
			TaskID:    task.ID,
			Status:    task.Status,
			PID:       task.PID,
			ExitCode:  task.ExitCode,
			Timestamp: time.Now().UnixMilli(),
		})

	case "list":
		tasks := h.tasks.List()
		data, _ := json.Marshal(tasks)
		resultTopic := fmt.Sprintf("agents/%s/results", h.agentID)
		h.client.Publish(resultTopic, 1, false, data)

	default:
		log.Printf("Unknown control action: %s", ctrl.Action)
	}
}

// publishResult publishes a result message to the results topic.
func (h *Handler) publishResult(result ResultMessage) {
	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("Failed to marshal result: %v", err)
		return
	}

	topic := fmt.Sprintf("agents/%s/results", h.agentID)
	h.client.Publish(topic, 1, false, data)
}

// publishRegistry publishes agent registration info to agents/registry (retained).
func (h *Handler) publishRegistry() {
	hostname, _ := os.Hostname()
	info := map[string]interface{}{
		"agentId":   h.agentID,
		"status":    "online",
		"platform":  runtime.GOOS,
		"arch":      runtime.GOARCH,
		"hostname":  hostname,
		"version":   "1.0.0",
		"timestamp": time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(info)
	h.client.Publish("agents/registry", 1, true, data)
	log.Printf("Published agent registry info")
}

// StartHeartbeat starts a goroutine that sends periodic heartbeat messages.
func (h *Handler) StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Send an initial heartbeat immediately
		h.sendHeartbeat()

		for {
			select {
			case <-ticker.C:
				h.sendHeartbeat()
			case <-h.stopHB:
				return
			}
		}
	}()
	log.Printf("Heartbeat started (interval=%v)", interval)
}

// StopHeartbeat stops the heartbeat goroutine.
func (h *Handler) StopHeartbeat() {
	close(h.stopHB)
}

// sendHeartbeat publishes a heartbeat message with system info.
func (h *Handler) sendHeartbeat() {
	hostname, _ := os.Hostname()
	hb := Heartbeat{
		Status:       "online",
		Platform:     runtime.GOOS,
		Arch:         runtime.GOARCH,
		Hostname:     hostname,
		Version:      "1.0.0",
		Uptime:       time.Since(h.startTime).Milliseconds(),
		RunningTasks: h.tasks.RunningCount(),
		Timestamp:    time.Now().UnixMilli(),
	}

	data, err := json.Marshal(hb)
	if err != nil {
		log.Printf("Failed to marshal heartbeat: %v", err)
		return
	}

	topic := fmt.Sprintf("agents/%s/heartbeat", h.agentID)
	h.client.Publish(topic, 1, true, data)
}

// handleDownload is a placeholder for file download handling.
// It will be fully implemented when file transfer is wired in (Task 9).
func (h *Handler) handleDownload(cmd Command) {
	h.publishResult(ResultMessage{
		TaskID:    cmd.ID,
		Status:    "error",
		Error:     "file download not yet implemented",
		Timestamp: time.Now().UnixMilli(),
	})
}
