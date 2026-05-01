package engine

import (
	"sync"
	"time"
)

type EventType string

const (
	EventWorkflowStarted  EventType = "workflow_started"
	EventNodeStarted      EventType = "node_started"
	EventNodeFinished     EventType = "node_finished"
	EventNodeError        EventType = "node_error"
	EventWorkflowFinished EventType = "workflow_finished"
	EventWorkflowFailed   EventType = "workflow_failed"
)

type WorkflowEvent struct {
	Event     EventType      `json:"event"`
	NodeID    string         `json:"node_id,omitempty"`
	NodeType  string         `json:"node_type,omitempty"`
	NodeTitle string         `json:"node_title,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Error     string         `json:"error,omitempty"`
	Elapsed   float64        `json:"elapsed_seconds"`
	CreatedAt int64          `json:"created_at"`
}

type EventEmitter struct {
	ch     chan WorkflowEvent
	closed bool
	mu     sync.Mutex
}

func NewEventEmitter(bufSize int) *EventEmitter {
	if bufSize <= 0 {
		bufSize = 64
	}
	return &EventEmitter{
		ch: make(chan WorkflowEvent, bufSize),
	}
}

func (e *EventEmitter) Emit(evt WorkflowEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	evt.CreatedAt = time.Now().UnixMilli()
	e.ch <- evt
}

func (e *EventEmitter) Events() <-chan WorkflowEvent {
	return e.ch
}

func (e *EventEmitter) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	close(e.ch)
}

func EmitNodeStarted(emitter *EventEmitter, nodeID, nodeType, title string) {
	emitter.Emit(WorkflowEvent{
		Event:     EventNodeStarted,
		NodeID:    nodeID,
		NodeType:  nodeType,
		NodeTitle: title,
	})
}

func EmitNodeFinished(emitter *EventEmitter, nodeID string, outputs map[string]any, elapsed float64) {
	emitter.Emit(WorkflowEvent{
		Event:   EventNodeFinished,
		NodeID:  nodeID,
		Data:    outputs,
		Elapsed: elapsed,
	})
}

func EmitNodeError(emitter *EventEmitter, nodeID, errMsg string, elapsed float64) {
	emitter.Emit(WorkflowEvent{
		Event:   EventNodeError,
		NodeID:  nodeID,
		Error:   errMsg,
		Elapsed: elapsed,
	})
}

func EmitWorkflowStarted(emitter *EventEmitter, workflowID string) {
	emitter.Emit(WorkflowEvent{
		Event: EventWorkflowStarted,
		Data:  map[string]any{"workflow_id": workflowID},
	})
}

func EmitWorkflowFinished(emitter *EventEmitter, outputs map[string]any, elapsed float64) {
	emitter.Emit(WorkflowEvent{
		Event:   EventWorkflowFinished,
		Data:    outputs,
		Elapsed: elapsed,
	})
}

func EmitWorkflowFailed(emitter *EventEmitter, errMsg string, elapsed float64) {
	emitter.Emit(WorkflowEvent{
		Event:   EventWorkflowFailed,
		Error:   errMsg,
		Elapsed: elapsed,
	})
}
