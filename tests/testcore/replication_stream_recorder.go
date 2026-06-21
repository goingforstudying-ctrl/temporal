package testcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"go.temporal.io/server/common/testing/testhooks"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Message direction constants
const (
	DirectionSend       = "send"
	DirectionRecv       = "recv"
	DirectionServerSend = "server_send"
	DirectionServerRecv = "server_recv"
)

// ReplicationStreamRecorder captures replication stream messages for testing
type ReplicationStreamRecorder struct {
	mu               sync.RWMutex
	capturedMessages []CapturedReplicationMessage
	outputFile       *os.File
	outputFilePath   string
}

// CapturedReplicationMessage represents a captured replication message
type CapturedReplicationMessage struct {
	Timestamp     string          `json:"timestamp"`
	Method        string          `json:"method"`
	Direction     string          `json:"direction"`
	ClusterName   string          `json:"clusterName"`
	TargetAddress string          `json:"targetAddress"`
	MessageType   string          `json:"messageType"`
	IsStreamCall  bool            `json:"isStreamCall"`
	Request       proto.Message   `json:"-"` // Don't marshal directly
	Response      proto.Message   `json:"-"` // Don't marshal directly
	Message       json.RawMessage `json:"message,omitempty"`
}

func NewReplicationStreamRecorder() *ReplicationStreamRecorder {
	return &ReplicationStreamRecorder{
		capturedMessages: make([]CapturedReplicationMessage, 0),
	}
}

// SetOutputFile sets the file path for writing captured messages on-demand
func (r *ReplicationStreamRecorder) SetOutputFile(filePath string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outputFilePath = filePath
}

// WriteToLog writes all captured messages to the configured output file
func (r *ReplicationStreamRecorder) WriteToLog() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.outputFilePath == "" {
		return errors.New("output file path not set")
	}

	// Create or truncate the output file
	f, err := os.Create(r.outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", r.outputFilePath, err)
	}
	defer func() {
		_ = f.Close()
	}()

	// Write all captured messages
	for _, captured := range r.capturedMessages {
		formattedMsg := r.formatCapturedMessage(captured)
		if _, err := f.WriteString(formattedMsg + "\n"); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	return f.Sync()
}

func (r *ReplicationStreamRecorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.capturedMessages = make([]CapturedReplicationMessage, 0)
}

func (r *ReplicationStreamRecorder) GetMessages() []CapturedReplicationMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]CapturedReplicationMessage, len(r.capturedMessages))
	copy(result, r.capturedMessages)
	return result
}

func (r *ReplicationStreamRecorder) Observe(message testhooks.ReplicationStreamMessage) {
	r.recordMessage(
		message.Method,
		message.Message,
		string(message.Direction),
		message.ClusterName,
		message.TargetAddress,
		message.IsStreamCall,
	)
}

func (r *ReplicationStreamRecorder) recordMessage(method string, msg proto.Message, direction string, clusterName string, targetAddr string, isStreamCall bool) {
	if msg == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	captured := CapturedReplicationMessage{
		Method:        method,
		Direction:     direction,
		ClusterName:   clusterName,
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		MessageType:   string(msg.ProtoReflect().Descriptor().FullName()),
		TargetAddress: targetAddr,
		IsStreamCall:  isStreamCall,
	}

	// Store the message reference directly without cloning for performance
	if direction == DirectionSend || direction == DirectionServerSend {
		captured.Request = msg
	} else {
		captured.Response = msg
	}

	r.capturedMessages = append(r.capturedMessages, captured)
}

// formatCapturedMessage formats a single captured message as JSON for output
func (r *ReplicationStreamRecorder) formatCapturedMessage(captured CapturedReplicationMessage) string {
	// Get the appropriate proto message based on direction
	var msg proto.Message
	if captured.Direction == DirectionSend || captured.Direction == DirectionServerSend {
		msg = captured.Request
	} else {
		msg = captured.Response
	}

	// Marshal the proto message to JSON and attach to Message field
	if msg != nil {
		marshaler := protojson.MarshalOptions{
			Multiline: false,
			Indent:    "",
		}
		jsonBytes, err := marshaler.Marshal(msg)
		if err == nil {
			captured.Message = jsonBytes
		}
	}

	// Marshal the entire struct to pretty JSON
	jsonOutput, err := json.MarshalIndent(captured, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal output: %v"}`, err)
	}

	return string(jsonOutput)
}
