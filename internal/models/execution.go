package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ExecutionStatus represents the status of a workflow execution
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// StepStatus represents the status of an individual execution step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// NodeType represents the type of node in the workflow
type NodeType string

const (
	NodeTypeStart       NodeType = "start"
	NodeTypeStartNode   NodeType = "startNode" // Frontend compatibility
	NodeTypeWebhook     NodeType = "webhook"
	NodeTypeWebhookNode NodeType = "webhookNode" // Frontend compatibility
	NodeTypeAI          NodeType = "ai"
	NodeTypeAINode      NodeType = "aiNode" // Frontend compatibility
	NodeTypeHTTP        NodeType = "http"
	NodeTypeHTTPNode    NodeType = "httpNode" // Frontend compatibility
	NodeTypeDelay       NodeType = "delay"
	NodeTypeDelayNode   NodeType = "delayNode" // Frontend compatibility
	NodeTypeEnd         NodeType = "end"
	NodeTypeEndNode     NodeType = "endNode" // Frontend compatibility
)

// WorkflowExecution represents a single execution of a workflow
type WorkflowExecution struct {
	ID             uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkflowID     uuid.UUID       `json:"workflowId" gorm:"type:uuid;not null;index"`
	Status         ExecutionStatus `json:"status" gorm:"type:varchar(50);not null;default:'pending'"`
	InputData      datatypes.JSON  `json:"inputData" gorm:"type:jsonb;default:'{}'"`
	OutputData     datatypes.JSON  `json:"outputData" gorm:"type:jsonb;default:'{}'"`
	ErrorMessage   string          `json:"errorMessage" gorm:"type:text"`
	StartedAt      *time.Time      `json:"startedAt"`
	CompletedAt    *time.Time      `json:"completedAt"`
	Workflow       Workflow        `json:"workflow" gorm:"foreignKey:WorkflowID"`
	ExecutionSteps []ExecutionStep `json:"executionSteps" gorm:"foreignKey:ExecutionID"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

func (e *WorkflowExecution) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// ExecutionStep represents a single step/node execution within a workflow execution
type ExecutionStep struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ExecutionID    uuid.UUID      `json:"executionId" gorm:"type:uuid;not null;index"`
	NodeID         string         `json:"nodeId" gorm:"type:varchar(255);not null"`
	NodeType       NodeType       `json:"nodeType" gorm:"type:varchar(50);not null"`
	Status         StepStatus     `json:"status" gorm:"type:varchar(50);not null;default:'pending'"`
	InputData      datatypes.JSON `json:"inputData" gorm:"type:jsonb;default:'{}'"`
	OutputData     datatypes.JSON `json:"outputData" gorm:"type:jsonb;default:'{}'"`
	ErrorMessage   string         `json:"errorMessage" gorm:"type:text"`
	StartedAt      *time.Time     `json:"startedAt"`
	CompletedAt    *time.Time     `json:"completedAt"`
	ExecutionOrder int            `json:"executionOrder" gorm:"not null"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

func (s *ExecutionStep) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// NodeConfig represents the configuration for a specific node
type NodeConfig struct {
	ID       string                 `json:"id"`
	Type     NodeType               `json:"type"`
	Config   map[string]interface{} `json:"config"`
	Position map[string]interface{} `json:"position,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"` // React Flow support
	Width    int                    `json:"width,omitempty"`
	Height   int                    `json:"height,omitempty"`
	Selected bool                   `json:"selected,omitempty"`
}

// GetConfig extracts the configuration from either Config or Data.Config
func (n *NodeConfig) GetConfig() map[string]interface{} {
	if n.Config != nil && len(n.Config) > 0 {
		return n.Config
	}
	if n.Data != nil {
		if config, ok := n.Data["config"].(map[string]interface{}); ok {
			return config
		}
	}
	return make(map[string]interface{})
}

// WorkflowNodes represents the nodes structure in the workflow JSON
type WorkflowNodes struct {
	Nodes []NodeConfig `json:"nodes"`
	Edges []EdgeConfig `json:"edges,omitempty"`
}

// EdgeConfig represents connections between nodes
type EdgeConfig struct {
	ID           string                 `json:"id"`
	Source       string                 `json:"source"`
	SourceHandle string                 `json:"sourceHandle,omitempty"` // For multiple output connections
	Target       string                 `json:"target"`
	TargetHandle string                 `json:"targetHandle,omitempty"` // For multiple input connections
	Condition    map[string]interface{} `json:"condition,omitempty"`    // Conditional routing
	Label        string                 `json:"label,omitempty"`        // Edge label for UI
	Type         string                 `json:"type,omitempty"`         // React Flow edge type
	Animated     bool                   `json:"animated,omitempty"`     // React Flow animation
	MarkerEnd    map[string]interface{} `json:"markerEnd,omitempty"`    // React Flow marker
	Style        map[string]interface{} `json:"style,omitempty"`        // React Flow style
	LabelStyle   map[string]interface{} `json:"labelStyle,omitempty"`   // React Flow label style
}

// WorkflowValidation represents validation results for a workflow
type WorkflowValidation struct {
	IsValid  bool     `json:"isValid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}
