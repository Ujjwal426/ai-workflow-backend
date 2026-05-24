package nodes

import (
	"context"
	"encoding/json"

	"gorm.io/datatypes"
)

// NodeExecutor defines the interface for executing different types of nodes
type NodeExecutor interface {
	Execute(ctx context.Context, input NodeInput) (NodeOutput, error)
	Validate(config map[string]interface{}) error
}

// NodeInput represents the input data for node execution
type NodeInput struct {
	NodeID   string                 `json:"nodeId"`
	NodeType string                 `json:"nodeType"`
	Config   map[string]interface{} `json:"config"`
	Data     map[string]interface{} `json:"data"`
}

// NodeOutput represents the output data from node execution
type NodeOutput struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
	Error   string                 `json:"error,omitempty"`
}

// ToJSON converts NodeOutput to JSON
func (o *NodeOutput) ToJSON() (datatypes.JSON, error) {
	return json.Marshal(o)
}

// FromJSON converts JSON to NodeOutput
func (o *NodeOutput) FromJSON(data datatypes.JSON) error {
	return json.Unmarshal(data, o)
}