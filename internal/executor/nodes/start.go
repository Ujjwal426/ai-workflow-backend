package nodes

import (
	"context"
	"errors"
	"fmt"
)

// StartNode represents the starting point of a workflow
type StartNode struct{}

// NewStartNode creates a new start node executor
func NewStartNode() *StartNode {
	return &StartNode{}
}

// Execute initializes the workflow execution with input data
func (n *StartNode) Execute(ctx context.Context, input NodeInput) (NodeOutput, error) {
	// Start node simply passes through the input data
	// It serves as the entry point for the workflow
	output := NodeOutput{
		Success: true,
		Data:    input.Data,
	}

	// Add start node metadata
	if input.Config != nil {
		if startData, ok := input.Config["startData"].(map[string]interface{}); ok {
			// Merge start data with input data
			for key, value := range startData {
				if _, exists := output.Data[key]; !exists {
					output.Data[key] = value
				}
			}
		}
	}

	return output, nil
}

// Validate validates start node configuration
func (n *StartNode) Validate(config map[string]interface{}) error {
	// Start node typically doesn't require configuration
	// But we can validate optional fields
	if config != nil {
		if startData, ok := config["startData"].(map[string]interface{}); ok {
			// Validate start data structure if needed
			for key, value := range startData {
				if key == "" {
					return errors.New("start data keys cannot be empty")
				}
				if value == nil {
					return errors.New("start data values cannot be null")
				}
			}
		}
	}

	return nil
}

// GetConfigSchema returns the expected configuration schema for start nodes
func (n *StartNode) GetConfigSchema() map[string]interface{} {
	return map[string]interface{}{
		"startData": map[string]interface{}{
			"type":        "object",
			"description": "Initial data to start the workflow with",
			"required":    false,
		},
		"description": map[string]interface{}{
			"type":        "string",
			"description": "Description of what this workflow starts",
			"required":    false,
		},
	}
}

func (n *StartNode) String() string {
	return fmt.Sprintf("StartNode")
}