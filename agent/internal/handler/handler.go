package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stevenyu113228/Clawpeteer/agent/internal/executor"
	"github.com/stevenyu113228/Clawpeteer/agent/internal/filetransfer"
	"github.com/stevenyu113228/Clawpeteer/agent/internal/security"
	"github.com/stevenyu113228/Clawpeteer/agent/internal/taskmanager"
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

// FileStatusMessage represents a file transfer status update.
type FileStatusMessage struct {
	TransferID     string  `json:"transferId"`
	Direction      string  `json:"direction"`
	Status         string  `json:"status"`
	ReceivedChunks int     `json:"receivedChunks,omitempty"`
	TotalChunks    int     `json:"totalChunks,omitempty"`
	Progress       float64 `json:"progress,omitempty"`
	Verified       bool    `json:"verified,omitempty"`
	Error          string  `json:"error,omitempty"`
	Timestamp      int64   `json:"timestamp"`
}

// UploadMeta represents incoming file upload metadata.
type UploadMeta struct {
	TransferID string `json:"transferId"`
	Filename   string `json:"filename"`
	DestPath   string `json:"destPath"`
	Size       int64  `json:"size"`
	TotalChunks int   `json:"totalChunks"`
	Sha256     string `json:"sha256"`
}

// UploadChunk represents an incoming file chunk.
type UploadChunk struct {
	TransferID string `json:"transferId"`
	Index      int    `json:"index"`
	Data       string `json:"data"`
}

// DownloadChunkMessage represents an outgoing download chunk.
type DownloadChunkMessage struct {
	TransferID string `json:"transferId"`
	Index      int    `json:"index"`
	Data       string `json:"data"`
}

// DownloadMetaMessage represents outgoing download metadata.
type DownloadMetaMessage struct {
	TransferID  string `json:"transferId"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	TotalChunks int    `json:"totalChunks"`
	Sha256      string `json:"sha256"`
	ChunkSize   int    `json:"chunkSize"`
}

// Handler orchestrates command execution, streaming, file transfer, and heartbeat.
type Handler struct {
	agentID      string
	client       mqtt.Client
	tasks        *taskmanager.Manager
	security     *security.Validator
	fileReceiver *filetransfer.Receiver
	processes    map[string]*executor.Process
	mu           sync.Mutex
	startTime    time.Time
	stopHB       chan struct{}
}

// New creates a new Handler.
func New(agentID string, client mqtt.Client, tasks *taskmanager.Manager, sec *security.Validator) *Handler {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp"
	}
	baseDir := filepath.Join(homeDir, ".clawpeteer")

	return &Handler{
		agentID:      agentID,
		client:       client,
		tasks:        tasks,
		security:     sec,
		fileReceiver: filetransfer.NewReceiver(baseDir),
		processes:    make(map[string]*executor.Process),
		startTime:    time.Now(),
		stopHB:       make(chan struct{}),
	}
}

// Subscribe subscribes to the command, control, and file transfer topics.
func (h *Handler) Subscribe() {
	commandTopic := fmt.Sprintf("agents/%s/commands", h.agentID)
	controlTopic := fmt.Sprintf("agents/%s/control/+", h.agentID)
	uploadMetaTopic := fmt.Sprintf("agents/%s/files/upload/+/meta", h.agentID)
	uploadChunkTopic := fmt.Sprintf("agents/%s/files/upload/+/chunks", h.agentID)

	h.client.Subscribe(commandTopic, 1, h.handleCommand)
	h.client.Subscribe(controlTopic, 1, h.handleControl)
	h.client.Subscribe(uploadMetaTopic, 1, h.handleUploadMeta)
	h.client.Subscribe(uploadChunkTopic, 1, h.handleUploadChunk)

	log.Printf("Subscribed to commands, control, and file transfer topics")

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

// handleUploadMeta handles incoming file upload metadata messages.
func (h *Handler) handleUploadMeta(client mqtt.Client, msg mqtt.Message) {
	var meta UploadMeta
	if err := json.Unmarshal(msg.Payload(), &meta); err != nil {
		log.Printf("Failed to parse upload meta: %v", err)
		return
	}

	log.Printf("Upload meta: transferId=%s filename=%s destPath=%s", meta.TransferID, meta.Filename, meta.DestPath)

	// Validate upload path with security
	if err := h.security.ValidateUploadPath(meta.DestPath); err != nil {
		h.publishFileStatus(FileStatusMessage{
			TransferID: meta.TransferID,
			Direction:  "upload",
			Status:     "error",
			Error:      fmt.Sprintf("security: %v", err),
			Timestamp:  time.Now().UnixMilli(),
		})
		return
	}

	// Initialize transfer in receiver
	if err := h.fileReceiver.InitTransfer(meta.TransferID, meta.Filename, meta.DestPath, meta.Size, meta.TotalChunks, meta.Sha256); err != nil {
		h.publishFileStatus(FileStatusMessage{
			TransferID: meta.TransferID,
			Direction:  "upload",
			Status:     "error",
			Error:      fmt.Sprintf("init: %v", err),
			Timestamp:  time.Now().UnixMilli(),
		})
		return
	}

	h.publishFileStatus(FileStatusMessage{
		TransferID:  meta.TransferID,
		Direction:   "upload",
		Status:      "receiving",
		TotalChunks: meta.TotalChunks,
		Timestamp:   time.Now().UnixMilli(),
	})
}

// handleUploadChunk handles incoming file chunk messages.
func (h *Handler) handleUploadChunk(client mqtt.Client, msg mqtt.Message) {
	var chunk UploadChunk
	if err := json.Unmarshal(msg.Payload(), &chunk); err != nil {
		log.Printf("Failed to parse upload chunk: %v", err)
		return
	}

	if err := h.fileReceiver.ReceiveChunk(chunk.TransferID, chunk.Index, chunk.Data); err != nil {
		log.Printf("ReceiveChunk error: %v", err)
		h.publishFileStatus(FileStatusMessage{
			TransferID: chunk.TransferID,
			Direction:  "upload",
			Status:     "error",
			Error:      fmt.Sprintf("chunk %d: %v", chunk.Index, err),
			Timestamp:  time.Now().UnixMilli(),
		})
		return
	}

	received, total, ok := h.fileReceiver.Progress(chunk.TransferID)
	if !ok {
		return
	}

	// Publish progress every 10%
	progress := float64(received) / float64(total) * 100
	progressStep := 100.0 / float64(total)
	if progressStep < 10 {
		progressStep = 10
	}
	// Report at every 10% boundary or when complete
	if int(progress)%10 == 0 || received == total {
		h.publishFileStatus(FileStatusMessage{
			TransferID:     chunk.TransferID,
			Direction:      "upload",
			Status:         "receiving",
			ReceivedChunks: received,
			TotalChunks:    total,
			Progress:       progress,
			Timestamp:      time.Now().UnixMilli(),
		})
	}

	// If all chunks received, finalize
	if received == total {
		verified, err := h.fileReceiver.Finalize(chunk.TransferID)
		if err != nil {
			h.publishFileStatus(FileStatusMessage{
				TransferID: chunk.TransferID,
				Direction:  "upload",
				Status:     "error",
				Error:      fmt.Sprintf("finalize: %v", err),
				Timestamp:  time.Now().UnixMilli(),
			})
			return
		}

		h.publishFileStatus(FileStatusMessage{
			TransferID:     chunk.TransferID,
			Direction:      "upload",
			Status:         "completed",
			ReceivedChunks: total,
			TotalChunks:    total,
			Progress:       100,
			Verified:       verified,
			Timestamp:      time.Now().UnixMilli(),
		})
	}
}

// handleDownload handles file download commands.
func (h *Handler) handleDownload(cmd Command) {
	// Validate download path
	if err := h.security.ValidateDownloadPath(cmd.SourcePath); err != nil {
		h.publishResult(ResultMessage{
			TaskID:    cmd.ID,
			Status:    "error",
			Error:     fmt.Sprintf("security: %v", err),
			Timestamp: time.Now().UnixMilli(),
		})
		return
	}

	const chunkSize = 64 * 1024 // 64KB chunks

	// Prepare download metadata
	meta, err := filetransfer.PrepareDownload(cmd.SourcePath, chunkSize)
	if err != nil {
		h.publishResult(ResultMessage{
			TaskID:    cmd.ID,
			Status:    "error",
			Error:     fmt.Sprintf("prepare download: %v", err),
			Timestamp: time.Now().UnixMilli(),
		})
		return
	}

	transferID := cmd.TransferID
	if transferID == "" {
		transferID = cmd.ID
	}

	// Publish metadata
	metaTopic := fmt.Sprintf("agents/%s/files/download/%s/meta", h.agentID, transferID)
	metaMsg := DownloadMetaMessage{
		TransferID:  transferID,
		Filename:    meta.Filename,
		Size:        meta.Size,
		TotalChunks: meta.TotalChunks,
		Sha256:      meta.Sha256,
		ChunkSize:   chunkSize,
	}
	metaData, _ := json.Marshal(metaMsg)
	h.client.Publish(metaTopic, 1, false, metaData)

	// Send all chunks
	chunkTopic := fmt.Sprintf("agents/%s/files/download/%s/chunks", h.agentID, transferID)
	for i := 0; i < meta.TotalChunks; i++ {
		encoded, err := filetransfer.ReadChunk(cmd.SourcePath, i, chunkSize)
		if err != nil {
			log.Printf("ReadChunk(%d) error: %v", i, err)
			h.publishFileStatus(FileStatusMessage{
				TransferID: transferID,
				Direction:  "download",
				Status:     "error",
				Error:      fmt.Sprintf("read chunk %d: %v", i, err),
				Timestamp:  time.Now().UnixMilli(),
			})
			return
		}

		chunkMsg := DownloadChunkMessage{
			TransferID: transferID,
			Index:      i,
			Data:       encoded,
		}
		chunkData, _ := json.Marshal(chunkMsg)
		h.client.Publish(chunkTopic, 0, false, chunkData)
	}

	// Publish completed status
	h.publishFileStatus(FileStatusMessage{
		TransferID:     transferID,
		Direction:      "download",
		Status:         "completed",
		ReceivedChunks: meta.TotalChunks,
		TotalChunks:    meta.TotalChunks,
		Progress:       100,
		Verified:       true,
		Timestamp:      time.Now().UnixMilli(),
	})
}

// publishFileStatus publishes a file transfer status message.
func (h *Handler) publishFileStatus(status FileStatusMessage) {
	data, err := json.Marshal(status)
	if err != nil {
		log.Printf("Failed to marshal file status: %v", err)
		return
	}

	topic := fmt.Sprintf("agents/%s/files/status", h.agentID)
	h.client.Publish(topic, 1, false, data)
}
