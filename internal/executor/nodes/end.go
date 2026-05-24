package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// EndNode represents the termination point of a workflow
type EndNode struct{}

// NewEndNode creates a new end node executor
func NewEndNode() *EndNode {
	return &EndNode{}
}

// Execute finalizes the workflow execution
func (n *EndNode) Execute(ctx context.Context, input NodeInput) (NodeOutput, error) {
	// End node marks the successful completion of the workflow
	// It collects the final output data from the workflow
	output := NodeOutput{
		Success: true,
		Data:    input.Data,
	}

	// Add end node metadata
	if input.Config != nil {
		// Mark as workflow completion
		output.Data["_workflowCompleted"] = true
		output.Data["_completionTime"] = fmt.Sprintf("%d", json.Number(fmt.Sprintf("%d", input.Data["timestamp"])))
		
		// Handle output transformation if configured
		if outputConfig, ok := input.Config["outputConfig"].(map[string]interface{}); ok {
			transformedData := make(map[string]interface{})
			
			// Apply output transformation rules
			if outputMapping, ok := outputConfig["mapping"].(map[string]interface{}); ok {
				for sourceKey, targetKey := range outputMapping {
					if sourceVal, exists := input.Data[sourceKey]; exists {
						if targetKeyStr, ok := targetKey.(string); ok {
							transformedData[targetKeyStr] = sourceVal
						}
					}
				}
			}
			
			if len(transformedData) > 0 {
				output.Data = transformedData
			}
		}
	}

	return output, nil
}

// Validate validates end node configuration
func (n *EndNode) Validate(config map[string]interface{}) error {
	// End node can have optional output transformation config
	if config != nil {
		if outputConfig, ok := config["outputConfig"].(map[string]interface{}); ok {
			if mapping, ok := outputConfig["mapping"].(map[string]interface{}); ok {
				// Validate mapping structure
				for sourceKey, targetKey := range mapping {
					if sourceKey == "" {
						return errors.New("mapping source keys cannot be empty")
					}
					if targetKey == "" {
						return errors.New("mapping target keys cannot be empty")
					}
				}
			}
		}
	}

	return nil
}

// GetConfigSchema returns the expected configuration schema for end nodes
func (n *EndNode) GetConfigSchema() map[string]interface{} {
	return map[string]interface{}{
		"outputConfig": map[string]interface{}{
			"type":        "object",
			"description": "Configuration for output data transformation",
			"required":    false,
			"properties": map[string]interface{}{
				"mapping": map[string]interface{}{
					"type":        "object",
					"description": "Map source keys to target keys for final output",
					"example":     map[string]interface{}{"userName": "name", "userEmail": "email"},
				},
			},
		},
		"description": map[string]interface{}{
			"type":        "string",
			"description": "Description of what this workflow accomplishes",
			"required":    false,
		},
		"returnResponse": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether to return response data to caller",
			"default":     true,
		},
	}
}

func (n *EndNode) String() string {
	return fmt.Sprintf("EndNode")
}