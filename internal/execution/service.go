package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ai-workflow-builder/internal/executor"
	"ai-workflow-builder/internal/executor/nodes"
	"ai-workflow-builder/internal/models"
	"ai-workflow-builder/internal/repository"
	"ai-workflow-builder/internal/service"
	"ai-workflow-builder/internal/websocket"

	"gorm.io/datatypes"
)

// Service handles workflow execution with websocket support
type Service struct {
	executionRepo repository.ExecutionRepository
	workflowRepo  repository.WorkflowRepository
	engine        *executor.Engine
	nodeRegistry  *nodes.NodeRegistry
	wsHub         *websocket.Hub
}

// NewService creates a new execution service with websocket support
func NewService(
	executionRepo repository.ExecutionRepository,
	workflowRepo repository.WorkflowRepository,
	nodeRegistry *nodes.NodeRegistry,
	wsHub *websocket.Hub,
) *Service {
	return &Service{
		executionRepo: executionRepo,
		workflowRepo:  workflowRepo,
		engine:        executor.NewEngine(executionRepo, nodeRegistry),
		nodeRegistry:  nodeRegistry,
		wsHub:         wsHub,
	}
}

// executeWorkflowInternal executes a workflow with real-time websocket updates (internal method)
func (s *Service) executeWorkflowInternal(ctx context.Context, workflow models.Workflow, inputData map[string]interface{}) (models.WorkflowExecution, error) {
	// Parse workflow JSON
	var workflowNodes models.WorkflowNodes
	if err := json.Unmarshal(workflow.WorkflowJSON, &workflowNodes); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	// Validate workflow structure
	if err := s.engine.ValidateWorkflowStructure(workflowNodes); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("invalid workflow structure: %w", err)
	}

	// Create workflow execution
	inputJSON, _ := json.Marshal(inputData)
	now := time.Now()
	execution := models.WorkflowExecution{
		WorkflowID: workflow.ID,
		Status:     models.ExecutionStatusPending,
		InputData:  datatypes.JSON(inputJSON),
		StartedAt:  &now,
	}

	if err := s.executionRepo.CreateExecution(ctx, &execution); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to create execution: %w", err)
	}

	// Send execution start event
	if s.wsHub != nil {
		s.wsHub.SendEvent(workflow.ID, websocket.NewExecutionStartEvent(execution.ID))
		s.wsHub.SendEvent(workflow.ID, websocket.NewExecutionLogEvent("Starting workflow execution..."))
	}

	// Update status to running
	execution.Status = models.ExecutionStatusRunning
	if err := s.executionRepo.UpdateExecution(ctx, &execution); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to update execution status: %w", err)
	}

	// Execute workflow with websocket broadcasting
	finalData, err := s.executeWorkflowGraphWithBroadcast(ctx, &execution, workflowNodes, inputData)
	if err != nil {
		execution.Status = models.ExecutionStatusFailed
		execution.ErrorMessage = err.Error()
		now := time.Now()
		execution.CompletedAt = &now
		s.executionRepo.UpdateExecution(ctx, &execution)

		// Send execution failed event
		if s.wsHub != nil {
			s.wsHub.SendEvent(workflow.ID, websocket.NewExecutionFailedEvent(execution.ID, err.Error()))
		}

		return execution, err
	}

	// Mark execution as completed
	execution.Status = models.ExecutionStatusCompleted
	outputJSON, _ := json.Marshal(finalData)
	execution.OutputData = datatypes.JSON(outputJSON)
	now = time.Now()
	execution.CompletedAt = &now

	if err := s.executionRepo.UpdateExecution(ctx, &execution); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to update execution status: %w", err)
	}

	// Send execution complete event
	if s.wsHub != nil {
		s.wsHub.SendEvent(workflow.ID, websocket.NewExecutionCompleteEvent(execution.ID, finalData))
		s.wsHub.SendEvent(workflow.ID, websocket.NewExecutionLogEvent("Workflow execution completed successfully"))
	}

	return execution, nil
}

// executeWorkflowGraphWithBroadcast executes the workflow following edge connections with websocket broadcasting
func (s *Service) executeWorkflowGraphWithBroadcast(ctx context.Context, execution *models.WorkflowExecution, workflowNodes models.WorkflowNodes, inputData map[string]interface{}) (map[string]interface{}, error) {
	// Build node lookup map
	nodeMap := make(map[string]models.NodeConfig)
	for _, node := range workflowNodes.Nodes {
		nodeMap[node.ID] = node
	}

	// Build adjacency list for edges
	edgeMap := make(map[string][]string)
	for _, edge := range workflowNodes.Edges {
		edgeMap[edge.Source] = append(edgeMap[edge.Source], edge.Target)
	}

	// Find start node
	startNode, err := s.engine.FindStartNode(workflowNodes.Nodes)
	if err != nil {
		return nil, err
	}

	// Execute nodes following the graph with broadcasting
	currentData := inputData
	visitedNodes := make(map[string]bool)
	executionOrder := 0

	return s.executeNodeRecursiveWithBroadcast(ctx, execution.ID, execution.WorkflowID, startNode, nodeMap, edgeMap, currentData, visitedNodes, &executionOrder)
}

// executeNodeRecursiveWithBroadcast recursively executes nodes with websocket broadcasting
func (s *Service) executeNodeRecursiveWithBroadcast(
	ctx context.Context,
	executionID uint,
	workflowID uint,
	currentNode models.NodeConfig,
	nodeMap map[string]models.NodeConfig,
	edgeMap map[string][]string,
	data map[string]interface{},
	visitedNodes map[string]bool,
	executionOrder *int,
) (map[string]interface{}, error) {
	// Check for cycles
	if visitedNodes[currentNode.ID] {
		return nil, fmt.Errorf("cycle detected at node %s", currentNode.ID)
	}
	visitedNodes[currentNode.ID] = true

	// Send node running event
	if s.wsHub != nil {
		s.wsHub.SendEvent(workflowID, websocket.NewNodeRunningEvent(currentNode.ID))
		nodeType := getNodeTypeName(currentNode.Type)
		s.wsHub.SendEvent(workflowID, websocket.NewExecutionLogEvent(fmt.Sprintf("Executing %s node (%s)...", nodeType, currentNode.ID)))
	}

	// Execute the current node
	step, err := s.executeNodeWithBroadcast(ctx, executionID, workflowID, currentNode, data, *executionOrder)
	if err != nil {
		// Send node error event
		if s.wsHub != nil {
			s.wsHub.SendEvent(workflowID, websocket.NewNodeErrorEvent(currentNode.ID, err.Error()))
		}
		return nil, err
	}
	*executionOrder++

	// Update data with node output
	if step.OutputData != nil {
		var outputMap map[string]interface{}
		if err := json.Unmarshal(step.OutputData, &outputMap); err == nil {
			if success, ok := outputMap["success"].(bool); ok && success {
				if newData, ok := outputMap["data"].(map[string]interface{}); ok {
					data = newData
				}
			}
		}
	}

	// Send node success event
	if s.wsHub != nil {
		s.wsHub.SendEvent(workflowID, websocket.NewNodeSuccessEvent(currentNode.ID, data))
	}

	// If this is an end node, return the data
	if currentNode.Type == models.NodeTypeEnd || currentNode.Type == models.NodeTypeEndNode {
		if s.wsHub != nil {
			s.wsHub.SendEvent(workflowID, websocket.NewExecutionLogEvent("Reached end node"))
		}
		return data, nil
	}

	// Get next nodes from edges
	nextNodeIds, hasNext := edgeMap[currentNode.ID]
	if !hasNext || len(nextNodeIds) == 0 {
		return data, nil
	}

	// Execute the next node (sequential for now)
	nextNodeId := nextNodeIds[0]
	nextNode, exists := nodeMap[nextNodeId]
	if !exists {
		return nil, fmt.Errorf("next node %s not found in node map", nextNodeId)
	}

	// Recursively execute the next node
	return s.executeNodeRecursiveWithBroadcast(ctx, executionID, workflowID, nextNode, nodeMap, edgeMap, data, visitedNodes, executionOrder)
}

// executeNodeWithBroadcast executes a single node with websocket broadcasting
func (s *Service) executeNodeWithBroadcast(ctx context.Context, executionID uint, workflowID uint, node models.NodeConfig, inputData map[string]interface{}, order int) (models.ExecutionStep, error) {
	// Get node executor
	nodeType := models.NodeType(node.Type)
	executor, err := s.nodeRegistry.Get(nodeType)
	if err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to get executor for node %s: %w", node.ID, err)
	}

	// Extract config
	nodeConfig := node.GetConfig()

	// Validate node configuration
	if err := executor.Validate(nodeConfig); err != nil {
		return models.ExecutionStep{}, fmt.Errorf("invalid config for node %s: %w", node.ID, err)
	}

	// Create execution step
	inputJSON, _ := json.Marshal(inputData)
	now := time.Now()
	step := models.ExecutionStep{
		ExecutionID:    executionID,
		NodeID:         node.ID,
		NodeType:       nodeType,
		Status:         models.StepStatusPending,
		InputData:      datatypes.JSON(inputJSON),
		ExecutionOrder: order,
		StartedAt:      &now,
	}

	if err := s.executionRepo.CreateStep(ctx, &step); err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to create execution step: %w", err)
	}

	// Update step status to running
	step.Status = models.StepStatusRunning
	if err := s.executionRepo.UpdateStep(ctx, &step); err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to update step status: %w", err)
	}

	// Execute node
	input := nodes.NodeInput{
		NodeID:   node.ID,
		NodeType: string(nodeType),
		Config:   nodeConfig,
		Data:     inputData,
	}

	output, err := executor.Execute(ctx, input)
	now = time.Now()
	step.CompletedAt = &now

	if err != nil {
		step.Status = models.StepStatusFailed
		step.ErrorMessage = err.Error()
		s.executionRepo.UpdateStep(ctx, &step)
		return step, fmt.Errorf("node execution failed: %w", err)
	}

	// Update step with output
	if output.Success {
		step.Status = models.StepStatusCompleted
	} else {
		step.Status = models.StepStatusFailed
		step.ErrorMessage = output.Error
	}

	outputJSON, _ := json.Marshal(output)
	step.OutputData = datatypes.JSON(outputJSON)

	if err := s.executionRepo.UpdateStep(ctx, &step); err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to update step: %w", err)
	}

	if !output.Success {
		return step, fmt.Errorf("node execution failed: %s", output.Error)
	}

	return step, nil
}

// ExecuteWorkflow executes a workflow by ID (implements service.ExecutionService interface)
func (s *Service) ExecuteWorkflow(ctx context.Context, workflowID uint, inputData map[string]interface{}) (models.WorkflowExecution, error) {
	// Get workflow
	workflow, err := s.workflowRepo.FindByID(ctx, workflowID)
	if err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to find workflow: %w", err)
	}

	return s.executeWorkflow(ctx, workflow, inputData)
}

// executeWorkflow is the internal implementation that takes a workflow object
func (s *Service) executeWorkflow(ctx context.Context, workflow models.Workflow, inputData map[string]interface{}) (models.WorkflowExecution, error) {
	return s.executeWorkflowInternal(ctx, workflow, inputData)
}

// ExecuteWorkflowDirect executes a workflow directly with nodes and edges (implements service.ExecutionService interface)
func (s *Service) ExecuteWorkflowDirect(ctx context.Context, input service.DirectExecutionInput) (models.WorkflowExecution, error) {
	// Create workflow object
	workflow := models.Workflow{
		ID: input.WorkflowID,
	}

	workflowNodes := models.WorkflowNodes{
		Nodes: input.Nodes,
		Edges: input.Edges,
	}

	workflowJSON, err := json.Marshal(workflowNodes)
	if err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to marshal workflow structure: %w", err)
	}

	workflow.WorkflowJSON = workflowJSON

	return s.executeWorkflowInternal(ctx, workflow, input.InputData)
}

// GetExecution retrieves an execution by ID
func (s *Service) GetExecution(ctx context.Context, id uint) (models.WorkflowExecution, error) {
	return s.executionRepo.FindByID(ctx, id)
}

// ListExecutions lists all executions for a workflow
func (s *Service) ListExecutions(ctx context.Context, workflowID uint) ([]models.WorkflowExecution, error) {
	return s.executionRepo.FindByWorkflowID(ctx, workflowID)
}

// ExecuteStep executes a single execution step
func (s *Service) ExecuteStep(ctx context.Context, executionID uint, stepID uint) (models.ExecutionStep, error) {
	// Get execution
	execution, err := s.executionRepo.FindByID(ctx, executionID)
	if err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to find execution: %w", err)
	}

	// Get step
	step, err := s.executionRepo.FindStepByID(ctx, stepID)
	if err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to find step: %w", err)
	}

	// Get workflow
	workflow, err := s.workflowRepo.FindByID(ctx, execution.WorkflowID)
	if err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to find workflow: %w", err)
	}

	// Parse workflow JSON
	var workflowNodes models.WorkflowNodes
	if err := json.Unmarshal(workflow.WorkflowJSON, &workflowNodes); err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	// Find node
	var node models.NodeConfig
	for _, n := range workflowNodes.Nodes {
		if n.ID == step.NodeID {
			node = n
			break
		}
	}

	// Re-execute the node
	inputData := make(map[string]interface{})
	if step.InputData != nil {
		json.Unmarshal(step.InputData, &inputData)
	}

	return s.executeNodeWithBroadcast(ctx, executionID, execution.WorkflowID, node, inputData, step.ExecutionOrder)
}

// GetExecutionSteps retrieves all steps for an execution
func (s *Service) GetExecutionSteps(ctx context.Context, executionID uint) ([]models.ExecutionStep, error) {
	return s.executionRepo.FindStepsByExecutionID(ctx, executionID)
}

// CancelExecution cancels a running execution
func (s *Service) CancelExecution(ctx context.Context, id uint) error {
	execution, err := s.executionRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find execution: %w", err)
	}

	// Only allow cancellation of running or pending executions
	if execution.Status != models.ExecutionStatusRunning && execution.Status != models.ExecutionStatusPending {
		return fmt.Errorf("can only cancel running or pending executions")
	}

	execution.Status = models.ExecutionStatusCancelled

	if err := s.executionRepo.UpdateExecution(ctx, &execution); err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	// Send cancellation event via WebSocket
	if s.wsHub != nil {
		s.wsHub.SendEvent(execution.WorkflowID, websocket.NewExecutionLogEvent("Execution cancelled"))
	}

	return nil
}

// getNodeTypeName returns a human-readable node type name
func getNodeTypeName(nodeType models.NodeType) string {
	switch nodeType {
	case models.NodeTypeStart, models.NodeTypeStartNode:
		return "Start"
	case models.NodeTypeWebhook:
		return "Webhook"
	case models.NodeTypeAI:
		return "AI"
	case models.NodeTypeHTTP:
		return "HTTP"
	case models.NodeTypeDelay, models.NodeTypeDelayNode:
		return "Delay"
	case models.NodeTypeEnd, models.NodeTypeEndNode:
		return "End"
	default:
		return string(nodeType)
	}
}
