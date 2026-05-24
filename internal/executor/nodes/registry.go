package nodes

import (
	"fmt"

	"ai-workflow-builder/internal/models"
)

// NodeRegistry manages available node executors
type NodeRegistry struct {
	executors map[models.NodeType]NodeExecutor
}

// NewNodeRegistry creates a new node registry with default executors
func NewNodeRegistry() *NodeRegistry {
	registry := &NodeRegistry{
		executors: make(map[models.NodeType]NodeExecutor),
	}

	// Register default node executors
	registry.Register(models.NodeTypeStart, NewStartNode())
	registry.Register(models.NodeTypeStartNode, NewStartNode()) // Frontend compatibility
	registry.Register(models.NodeTypeWebhook, NewWebhookNode())
	registry.Register(models.NodeTypeWebhookNode, NewWebhookNode()) // Frontend compatibility
	registry.Register(models.NodeTypeAI, NewAINode())
	registry.Register(models.NodeTypeAINode, NewAINode()) // Frontend compatibility
	registry.Register(models.NodeTypeHTTP, NewHTTPNode())
	registry.Register(models.NodeTypeHTTPNode, NewHTTPNode()) // Frontend compatibility
	registry.Register(models.NodeTypeDelay, NewDelayNode())
	registry.Register(models.NodeTypeDelayNode, NewDelayNode()) // Frontend compatibility
	registry.Register(models.NodeTypeEnd, NewEndNode())
	registry.Register(models.NodeTypeEndNode, NewEndNode()) // Frontend compatibility

	return registry
}

// Register registers a new node executor
func (r *NodeRegistry) Register(nodeType models.NodeType, executor NodeExecutor) {
	r.executors[nodeType] = executor
}

// Get retrieves a node executor by type
func (r *NodeRegistry) Get(nodeType models.NodeType) (NodeExecutor, error) {
	executor, ok := r.executors[nodeType]
	if !ok {
		return nil, fmt.Errorf("no executor registered for node type: %s", nodeType)
	}
	return executor, nil
}

// GetSupportedTypes returns all supported node types
func (r *NodeRegistry) GetSupportedTypes() []models.NodeType {
	types := make([]models.NodeType, 0, len(r.executors))
	for nodeType := range r.executors {
		types = append(types, nodeType)
	}
	return types
}
