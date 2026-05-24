package nodes

import (
	"context"
	"errors"
	"fmt"
)

// WebhookNode handles webhook trigger nodes
type WebhookNode struct{}

// NewWebhookNode creates a new webhook node executor
func NewWebhookNode() *WebhookNode {
	return &WebhookNode{}
}

// Execute executes a webhook node (simulated - in real scenario, this would handle incoming webhook)
func (n *WebhookNode) Execute(ctx context.Context, input NodeInput) (NodeOutput, error) {
	// Webhook nodes are typically trigger nodes that receive external data
	// For execution purposes, we'll pass through the input data
	output := NodeOutput{
		Success: true,
		Data:    input.Data,
	}

	// Add webhook-specific metadata
	if input.Config != nil {
		if webhookURL, ok := input.Config["url"].(string); ok {
			output.Data["webhookUrl"] = webhookURL
		}
		if method, ok := input.Config["method"].(string); ok {
			output.Data["method"] = method
		}
	}

	return output, nil
}

// Validate validates webhook node configuration
func (n *WebhookNode) Validate(config map[string]interface{}) error {
	if config == nil {
		return errors.New("webhook config is required")
	}

	// In a real implementation, you might validate webhook URL format
	if url, ok := config["url"].(string); ok && url == "" {
		return errors.New("webhook URL cannot be empty")
	}

	return nil
}

// GetConfigSchema returns the expected configuration schema for webhook nodes
func (n *WebhookNode) GetConfigSchema() map[string]interface{} {
	return map[string]interface{}{
		"url": map[string]interface{}{
			"type":        "string",
			"description": "Webhook URL",
			"required":    true,
		},
		"method": map[string]interface{}{
			"type":        "string",
			"description": "HTTP method (GET, POST, etc.)",
			"default":     "POST",
		},
		"headers": map[string]interface{}{
			"type":        "object",
			"description": "HTTP headers",
		},
	}
}

func (n *WebhookNode) String() string {
	return fmt.Sprintf("WebhookNode")
}