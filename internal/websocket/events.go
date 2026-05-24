package websocket

// EventType represents the type of websocket event
type EventType string

const (
	// Node events
	EventNodeRunning EventType = "NODE_RUNNING"
	EventNodeSuccess EventType = "NODE_SUCCESS"
	EventNodeError   EventType = "NODE_ERROR"
	
	// Execution events
	EventExecutionLog EventType = "EXECUTION_LOG"
	EventExecutionStart EventType = "EXECUTION_START"
	EventExecutionComplete EventType = "EXECUTION_COMPLETE"
	EventExecutionFailed EventType = "EXECUTION_FAILED"
)

// WSEvent represents a websocket event
type WSEvent struct {
	Type    EventType                 `json:"type"`
	NodeID  string                    `json:"nodeId,omitempty"`
	Message string                    `json:"message,omitempty"`
	Data    map[string]interface{}   `json:"data,omitempty"`
}

// NewNodeRunningEvent creates a NODE_RUNNING event
func NewNodeRunningEvent(nodeID string) *WSEvent {
	return &WSEvent{
		Type:   EventNodeRunning,
		NodeID: nodeID,
	}
}

// NewNodeSuccessEvent creates a NODE_SUCCESS event
func NewNodeSuccessEvent(nodeID string, data map[string]interface{}) *WSEvent {
	return &WSEvent{
		Type:   EventNodeSuccess,
		NodeID: nodeID,
		Data:   data,
	}
}

// NewNodeErrorEvent creates a NODE_ERROR event
func NewNodeErrorEvent(nodeID, message string) *WSEvent {
	return &WSEvent{
		Type:    EventNodeError,
		NodeID:  nodeID,
		Message: message,
	}
}

// NewExecutionLogEvent creates an EXECUTION_LOG event
func NewExecutionLogEvent(message string) *WSEvent {
	return &WSEvent{
		Type:    EventExecutionLog,
		Message: message,
	}
}

// NewExecutionStartEvent creates an EXECUTION_START event
func NewExecutionStartEvent(executionID uint) *WSEvent {
	return &WSEvent{
		Type: EventExecutionStart,
		Data: map[string]interface{}{
			"executionId": executionID,
		},
	}
}

// NewExecutionCompleteEvent creates an EXECUTION_COMPLETE event
func NewExecutionCompleteEvent(executionID uint, data map[string]interface{}) *WSEvent {
	return &WSEvent{
		Type: EventExecutionComplete,
		Data: map[string]interface{}{
			"executionId": executionID,
			"outputData":  data,
		},
	}
}

// NewExecutionFailedEvent creates an EXECUTION_FAILED event
func NewExecutionFailedEvent(executionID uint, message string) *WSEvent {
	return &WSEvent{
		Type:    EventExecutionFailed,
		Data:    map[string]interface{}{"executionId": executionID},
		Message: message,
	}
}