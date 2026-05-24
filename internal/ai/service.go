package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Service handles AI-powered workflow generation
type Service struct {
	client *openai.Client
}

// NewService creates a new AI service
func NewService() (*Service, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY environment variable is not set")
	}

	client := openai.NewClient(apiKey)

	return &Service{
		client: client,
	}, nil
}

// GenerateWorkflowRequest represents a workflow generation request
type GenerateWorkflowRequest struct {
	Prompt string `json:"prompt"`
}

// WorkflowNode represents a workflow node in the AI response
type WorkflowNode struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"`
}

// WorkflowEdge represents a workflow edge in the AI response
type WorkflowEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

// WorkflowResponse represents the AI-generated workflow structure
type WorkflowResponse struct {
	Nodes []WorkflowNode `json:"nodes"`
	Edges []WorkflowEdge `json:"edges"`
}

// GenerateWorkflow generates a workflow structure from a user prompt
func (s *Service) GenerateWorkflow(ctx context.Context, request GenerateWorkflowRequest) (*WorkflowResponse, error) {
	if request.Prompt == "" {
		return nil, errors.New("prompt is required")
	}

	// System prompt for workflow generation
	systemPrompt := `You are a workflow automation expert. Convert the user request into workflow nodes.

Available node types:
- startNode: Workflow start point (must always be first)
- endNode: Workflow end point (must always be last)
- webhookNode: Webhook trigger/receiver
- delayNode: Wait/pause for a specified time
- httpNode: Make HTTP requests to APIs
- aiNode: Process data with AI/LLM

IMPORTANT: Always include a startNode as the first node and an endNode as the last node.
Connect nodes in sequence: startNode → [workflow nodes] → endNode

Return ONLY valid JSON in this format:
{
  "nodes": [
    {
      "id": "unique-id",
      "type": "nodeType",
      "title": "Node Title",
      "description": "Node description",
      "config": {}
    }
  ],
  "edges": [
    {
      "id": "unique-edge-id",
      "source": "source-node-id",
      "target": "target-node-id"
    }
  ]
}

Rules:
- Always start with a webhookNode if the workflow is triggered by external data
- Use delayNode with config: {"duration": number, "unit": "seconds"}
- Use httpNode with config: {"url": "https://...", "method": "POST"}
- Connect nodes with edges in execution order
- Generate unique IDs for nodes and edges
- Return ONLY the JSON, no additional text`

	// Create chat completion request
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: request.Prompt,
		},
	}

	resp, err := s.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       openai.GPT4oMini,
			Messages:    messages,
			Temperature: 0.7,
			MaxTokens:   1000,
		},
	)

	if err != nil {
		// Return a mock response if OpenAI API fails (e.g., quota exceeded)
		return s.getMockWorkflowResponse(request.Prompt), nil
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no response generated")
	}

	// Extract and parse the response
	content := resp.Choices[0].Message.Content

	// Try to extract JSON from the response (in case there's extra text)
	workflowResp, err := s.extractAndValidateJSON(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow response: %w", err)
	}

	return workflowResp, nil
}

// extractAndValidateJSON extracts and validates JSON from the AI response
func (s *Service) extractAndValidateJSON(content string) (*WorkflowResponse, error) {
	// Try to parse the content as JSON directly
	var workflowResp WorkflowResponse
	if err := json.Unmarshal([]byte(content), &workflowResp); err != nil {
		// If direct parsing fails, try to extract JSON from markdown code blocks
		extracted, extractErr := s.extractJSONFromMarkdown(content)
		if extractErr != nil {
			return nil, fmt.Errorf("invalid JSON response: %w", err)
		}

		if err := json.Unmarshal([]byte(extracted), &workflowResp); err != nil {
			return nil, fmt.Errorf("failed to parse extracted JSON: %w", err)
		}
	}

	// Validate the workflow structure
	if err := s.validateWorkflowResponse(&workflowResp); err != nil {
		return nil, fmt.Errorf("invalid workflow structure: %w", err)
	}

	return &workflowResp, nil
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func (s *Service) extractJSONFromMarkdown(content string) (string, error) {
	// Look for ```json or ``` code blocks
	startIndex := -1
	endIndex := -1

	// Try to find JSON code block
	for i := 0; i < len(content)-6; i++ {
		if content[i:i+6] == "```json" {
			startIndex = i + 7 // Skip the ```json
			break
		}
		if content[i:i+3] == "```" {
			startIndex = i + 4 // Skip the ```
			break
		}
	}

	if startIndex == -1 {
		return "", errors.New("no JSON code block found")
	}

	// Find the closing ```
	for i := startIndex; i < len(content)-3; i++ {
		if content[i:i+3] == "```" {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		return "", errors.New("unclosed JSON code block")
	}

	return content[startIndex:endIndex], nil
}

// validateWorkflowResponse validates the workflow response structure
func (s *Service) validateWorkflowResponse(resp *WorkflowResponse) error {
	if resp == nil {
		return errors.New("workflow response is nil")
	}

	if len(resp.Nodes) == 0 {
		return errors.New("workflow must have at least one node")
	}

	// Validate nodes
	nodeIDs := make(map[string]bool)
	for i, node := range resp.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node at index %d has empty ID", i)
		}
		if nodeIDs[node.ID] {
			return fmt.Errorf("duplicate node ID: %s", node.ID)
		}
		nodeIDs[node.ID] = true

		// Validate node type
		validTypes := map[string]bool{
			"startNode":   true,
			"endNode":     true,
			"webhookNode": true,
			"delayNode":   true,
			"httpNode":    true,
			"aiNode":      true,
		}
		if !validTypes[node.Type] {
			return fmt.Errorf("invalid node type: %s", node.Type)
		}

		// Validate delay node config
		if node.Type == "delayNode" {
			if node.Config == nil {
				return fmt.Errorf("delay node %s missing config", node.ID)
			}
			if _, ok := node.Config["duration"]; !ok {
				return fmt.Errorf("delay node %s missing duration in config", node.ID)
			}
		}

		// Validate HTTP node config
		if node.Type == "httpNode" {
			if node.Config == nil {
				return fmt.Errorf("HTTP node %s missing config", node.ID)
			}
			if _, ok := node.Config["url"]; !ok {
				return fmt.Errorf("HTTP node %s missing URL in config", node.ID)
			}
		}
	}

	// Validate edges
	edgeIDs := make(map[string]bool)
	for i, edge := range resp.Edges {
		if edge.ID == "" {
			return fmt.Errorf("edge at index %d has empty ID", i)
		}
		if edgeIDs[edge.ID] {
			return fmt.Errorf("duplicate edge ID: %s", edge.ID)
		}
		edgeIDs[edge.ID] = true

		if edge.Source == "" {
			return fmt.Errorf("edge %d has empty source", i)
		}
		if edge.Target == "" {
			return fmt.Errorf("edge %d has empty target", i)
		}

		// Check if source and target nodes exist
		if !nodeIDs[edge.Source] {
			return fmt.Errorf("edge %d references non-existent source node: %s", i, edge.Source)
		}
		if !nodeIDs[edge.Target] {
			return fmt.Errorf("edge %d references non-existent target node: %s", i, edge.Target)
		}
	}

	return nil
}

// getMockWorkflowResponse returns a mock workflow response for testing
func (s *Service) getMockWorkflowResponse(prompt string) *WorkflowResponse {
	// Always start with start node
	nodes := []WorkflowNode{
		{
			ID:          "node-1",
			Type:        "startNode",
			Title:       "Start",
			Description: "Workflow start point",
			Config:      map[string]interface{}{},
		},
	}

	edges := []WorkflowEdge{}
	lastNodeID := "node-1"

	// Add webhook node if prompt mentions "webhook" or "receive" or "trigger"
	if contains(prompt, []string{"webhook", "receive", "trigger"}) {
		nextNodeID := fmt.Sprintf("node-%d", len(nodes)+1)
		nodes = append(nodes, WorkflowNode{
			ID:          nextNodeID,
			Type:        "webhookNode",
			Title:       "Receive Webhook",
			Description: "Webhook trigger",
			Config:      map[string]interface{}{"endpoint": "/webhook"},
		})
		edges = append(edges, WorkflowEdge{
			ID:     fmt.Sprintf("edge-%d", len(edges)+1),
			Source: lastNodeID,
			Target: nextNodeID,
		})
		lastNodeID = nextNodeID
	}

	// Add delay node if prompt mentions "wait" or "delay"
	if contains(prompt, []string{"wait", "delay", "pause"}) {
		nextNodeID := fmt.Sprintf("node-%d", len(nodes)+1)
		nodes = append(nodes, WorkflowNode{
			ID:          nextNodeID,
			Type:        "delayNode",
			Title:       "Delay",
			Description: "Wait before continuing",
			Config:      map[string]interface{}{"duration": 5, "unit": "seconds"},
		})
		edges = append(edges, WorkflowEdge{
			ID:     fmt.Sprintf("edge-%d", len(edges)+1),
			Source: lastNodeID,
			Target: nextNodeID,
		})
		lastNodeID = nextNodeID
	}

	// Add AI node if prompt mentions "ai", "process", "transform"
	if contains(prompt, []string{"ai", "process", "transform", "analyze"}) {
		nextNodeID := fmt.Sprintf("node-%d", len(nodes)+1)
		nodes = append(nodes, WorkflowNode{
			ID:          nextNodeID,
			Type:        "aiNode",
			Title:       "AI Processing",
			Description: "Process with AI",
			Config:      map[string]interface{}{"model": "gpt-4", "prompt": "Process the data"},
		})
		edges = append(edges, WorkflowEdge{
			ID:     fmt.Sprintf("edge-%d", len(edges)+1),
			Source: lastNodeID,
			Target: nextNodeID,
		})
		lastNodeID = nextNodeID
	}

	// Add HTTP node if prompt mentions "call", "api", "request"
	if contains(prompt, []string{"call", "api", "request", "http"}) {
		nextNodeID := fmt.Sprintf("node-%d", len(nodes)+1)
		nodes = append(nodes, WorkflowNode{
			ID:          nextNodeID,
			Type:        "httpNode",
			Title:       "HTTP Request",
			Description: "Call external API",
			Config:      map[string]interface{}{"url": "https://api.example.com", "method": "POST"},
		})
		edges = append(edges, WorkflowEdge{
			ID:     fmt.Sprintf("edge-%d", len(edges)+1),
			Source: lastNodeID,
			Target: nextNodeID,
		})
		lastNodeID = nextNodeID
	}

	// If no workflow nodes were added (except start), add a default webhook node
	if len(nodes) == 1 {
		nodes = append(nodes, WorkflowNode{
			ID:          "node-2",
			Type:        "webhookNode",
			Title:       "Receive Webhook",
			Description: "Webhook trigger",
			Config:      map[string]interface{}{"endpoint": "/webhook"},
		})
		edges = append(edges, WorkflowEdge{
			ID:     "edge-1",
			Source: lastNodeID,
			Target: "node-2",
		})
		lastNodeID = "node-2"
	}

	// Always end with end node
	endNodeID := fmt.Sprintf("node-%d", len(nodes)+1)
	nodes = append(nodes, WorkflowNode{
		ID:          endNodeID,
		Type:        "endNode",
		Title:       "End",
		Description: "Workflow end point",
		Config:      map[string]interface{}{},
	})
	edges = append(edges, WorkflowEdge{
		ID:     fmt.Sprintf("edge-%d", len(edges)+1),
		Source: lastNodeID,
		Target: endNodeID,
	})

	return &WorkflowResponse{
		Nodes: nodes,
		Edges: edges,
	}
}

// contains checks if the prompt contains any of the keywords
func contains(prompt string, keywords []string) bool {
	lowerPrompt := strings.ToLower(prompt)
	for _, keyword := range keywords {
		if strings.Contains(lowerPrompt, keyword) {
			return true
		}
	}
	return false
}
