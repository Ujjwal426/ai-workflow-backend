package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPNode handles HTTP request nodes
type HTTPNode struct {
	client *http.Client
}

// NewHTTPNode creates a new HTTP node executor
func NewHTTPNode() *HTTPNode {
	return &HTTPNode{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute executes an HTTP node
func (n *HTTPNode) Execute(ctx context.Context, input NodeInput) (NodeOutput, error) {
	if input.Config == nil {
		return NodeOutput{}, errors.New("HTTP config is required")
	}

	// Extract HTTP configuration
	url, ok := input.Config["url"].(string)
	if !ok || url == "" {
		return NodeOutput{}, errors.New("HTTP URL is required")
	}

	method := "POST"
	if m, ok := input.Config["method"].(string); ok && m != "" {
		method = m
	}

	// Prepare request body
	var body io.Reader
	if method == "POST" || method == "PUT" || method == "PATCH" {
		if input.Data != nil && len(input.Data) > 0 {
			jsonBody, err := json.Marshal(input.Data)
			if err != nil {
				return NodeOutput{}, fmt.Errorf("failed to marshal request body: %w", err)
			}
			body = bytes.NewReader(jsonBody)
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if headers, ok := input.Config["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Set(key, strValue)
			}
		}
	}

	// Execute request
	resp, err := n.client.Do(req)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return NodeOutput{}, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response JSON if possible
	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		// If not JSON, return as string
		respData = map[string]interface{}{
			"raw": string(respBody),
		}
	}

	output := NodeOutput{
		Success: resp.StatusCode >= 200 && resp.StatusCode < 300,
		Data: map[string]interface{}{
			"statusCode": resp.StatusCode,
			"status":     resp.Status,
			"headers":    resp.Header,
			"body":       respData,
			"url":        url,
			"method":     method,
		},
	}

	if !output.Success {
		output.Error = fmt.Sprintf("HTTP request failed with status %d", resp.StatusCode)
	}

	return output, nil
}

// Validate validates HTTP node configuration
func (n *HTTPNode) Validate(config map[string]interface{}) error {
	if config == nil {
		return errors.New("HTTP config is required")
	}

	if url, ok := config["url"].(string); !ok || url == "" {
		return errors.New("HTTP URL is required")
	}

	return nil
}

// GetConfigSchema returns the expected configuration schema for HTTP nodes
func (n *HTTPNode) GetConfigSchema() map[string]interface{} {
	return map[string]interface{}{
		"url": map[string]interface{}{
			"type":        "string",
			"description": "HTTP request URL",
			"required":    true,
		},
		"method": map[string]interface{}{
			"type":        "string",
			"description": "HTTP method (GET, POST, PUT, DELETE, etc.)",
			"default":     "POST",
		},
		"headers": map[string]interface{}{
			"type":        "object",
			"description": "HTTP headers",
		},
		"timeout": map[string]interface{}{
			"type":        "number",
			"description": "Request timeout in seconds",
			"default":     30,
		},
	}
}

func (n *HTTPNode) String() string {
	return fmt.Sprintf("HTTPNode")
}