package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ai-workflow-builder/internal/executor/nodes"
	"ai-workflow-builder/internal/models"
	"ai-workflow-builder/internal/repository"

	"gorm.io/datatypes"
)

// Engine handles workflow execution orchestration
type Engine struct {
	executionRepo repository.ExecutionRepository
	nodeRegistry  *nodes.NodeRegistry
}

// NewEngine creates a new workflow execution engine
func NewEngine(executionRepo repository.ExecutionRepository, nodeRegistry *nodes.NodeRegistry) *Engine {
	return &Engine{
		executionRepo: executionRepo,
		nodeRegistry:  nodeRegistry,
	}
}

// ExecuteWorkflow executes a workflow with the given input data
func (e *Engine) ExecuteWorkflow(ctx context.Context, workflow models.Workflow, inputData map[string]interface{}) (models.WorkflowExecution, error) {
	// Parse workflow JSON to get nodes
	var workflowNodes models.WorkflowNodes
	if err := json.Unmarshal(workflow.WorkflowJSON, &workflowNodes); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	// Validate workflow structure
	if err := e.validateWorkflowStructure(workflowNodes); err != nil {
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

	if err := e.executionRepo.CreateExecution(ctx, &execution); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to create execution: %w", err)
	}

	// Update status to running
	execution.Status = models.ExecutionStatusRunning
	if err := e.executionRepo.UpdateExecution(ctx, &execution); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to update execution status: %w", err)
	}

	// Execute workflow following edge connections
	finalData, err := e.executeWorkflowGraph(ctx, &execution, workflowNodes, inputData)
	if err != nil {
		execution.Status = models.ExecutionStatusFailed
		execution.ErrorMessage = err.Error()
		now := time.Now()
		execution.CompletedAt = &now
		e.executionRepo.UpdateExecution(ctx, &execution)
		return execution, err
	}

	// Mark execution as completed
	execution.Status = models.ExecutionStatusCompleted
	outputJSON, _ := json.Marshal(finalData)
	execution.OutputData = datatypes.JSON(outputJSON)
	now = time.Now()
	execution.CompletedAt = &now

	if err := e.executionRepo.UpdateExecution(ctx, &execution); err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to update execution status: %w", err)
	}

	return execution, nil
}

// executeWorkflowGraph executes the workflow following edge connections
func (e *Engine) executeWorkflowGraph(ctx context.Context, execution *models.WorkflowExecution, workflowNodes models.WorkflowNodes, inputData map[string]interface{}) (map[string]interface{}, error) {
	// Build node lookup map
	nodeMap := make(map[string]models.NodeConfig)
	for _, node := range workflowNodes.Nodes {
		nodeMap[node.ID] = node
	}

	// Build adjacency list for edges
	edgeMap := make(map[string][]string) // source -> targets
	for _, edge := range workflowNodes.Edges {
		edgeMap[edge.Source] = append(edgeMap[edge.Source], edge.Target)
	}

	// Find start node
	startNode, err := e.findStartNode(workflowNodes.Nodes)
	if err != nil {
		return nil, err
	}

	// Execute nodes following the graph
	currentData := inputData
	visitedNodes := make(map[string]bool)
	executionOrder := 0

	return e.executeNodeRecursive(ctx, execution.ID, startNode, nodeMap, edgeMap, currentData, visitedNodes, &executionOrder)
}

// executeNodeRecursive recursively executes nodes following edges
func (e *Engine) executeNodeRecursive(
	ctx context.Context,
	executionID uint,
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

	// Execute the current node
	step, err := e.executeNode(ctx, executionID, currentNode, data, *executionOrder)
	if err != nil {
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

	// If this is an end node, return the data
	if currentNode.Type == models.NodeTypeEnd || currentNode.Type == models.NodeTypeEndNode {
		return data, nil
	}

	// Get next nodes from edges
	nextNodeIds, hasNext := edgeMap[currentNode.ID]
	if !hasNext || len(nextNodeIds) == 0 {
		// No more nodes, return current data
		return data, nil
	}

	// For now, execute the first connected node (sequential execution)
	// TODO: Support parallel execution for multiple connections
	nextNodeId := nextNodeIds[0]
	nextNode, exists := nodeMap[nextNodeId]
	if !exists {
		return nil, fmt.Errorf("next node %s not found in node map", nextNodeId)
	}

	// Recursively execute the next node
	return e.executeNodeRecursive(ctx, executionID, nextNode, nodeMap, edgeMap, data, visitedNodes, executionOrder)
}

// FindStartNode finds the start node in the workflow (public method)
func (e *Engine) FindStartNode(nodes []models.NodeConfig) (models.NodeConfig, error) {
	return e.findStartNode(nodes)
}

// findStartNode finds the start node in the workflow
func (e *Engine) findStartNode(nodes []models.NodeConfig) (models.NodeConfig, error) {
	for _, node := range nodes {
		if node.Type == models.NodeTypeStart || node.Type == models.NodeTypeStartNode {
			return node, nil
		}
	}
	return models.NodeConfig{}, fmt.Errorf("no start node found in workflow")
}

// ValidateWorkflowStructure validates the workflow structure (public method)
func (e *Engine) ValidateWorkflowStructure(workflowNodes models.WorkflowNodes) error {
	return e.validateWorkflowStructure(workflowNodes)
}

// validateWorkflowStructure validates the workflow structure
func (e *Engine) validateWorkflowStructure(workflowNodes models.WorkflowNodes) error {
	if len(workflowNodes.Nodes) == 0 {
		return fmt.Errorf("workflow must have at least one node")
	}

	// Check for start node (support both "start" and "startNode")
	hasStart := false
	hasEnd := false
	for _, node := range workflowNodes.Nodes {
		if node.Type == models.NodeTypeStart || node.Type == models.NodeTypeStartNode {
			hasStart = true
		}
		if node.Type == models.NodeTypeEnd || node.Type == models.NodeTypeEndNode {
			hasEnd = true
		}
	}

	if !hasStart {
		return fmt.Errorf("workflow must have a start node")
	}

	if !hasEnd {
		return fmt.Errorf("workflow must have an end node")
	}

	// Check for valid edge connections
	nodeIds := make(map[string]bool)
	for _, node := range workflowNodes.Nodes {
		nodeIds[node.ID] = true
	}

	for _, edge := range workflowNodes.Edges {
		if !nodeIds[edge.Source] {
			return fmt.Errorf("edge source node %s does not exist", edge.Source)
		}
		if !nodeIds[edge.Target] {
			return fmt.Errorf("edge target node %s does not exist", edge.Target)
		}
	}

	return nil
}

// executeNode executes a single node
func (e *Engine) executeNode(ctx context.Context, executionID uint, node models.NodeConfig, inputData map[string]interface{}, order int) (models.ExecutionStep, error) {
	// Get node executor
	nodeType := models.NodeType(node.Type)
	executor, err := e.nodeRegistry.Get(nodeType)
	if err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to get executor for node %s: %w", node.ID, err)
	}

	// Extract config from node (handles both Config and Data.Config)
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

	if err := e.executionRepo.CreateStep(ctx, &step); err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to create execution step: %w", err)
	}

	// Update step status to running
	step.Status = models.StepStatusRunning
	if err := e.executionRepo.UpdateStep(ctx, &step); err != nil {
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
		e.executionRepo.UpdateStep(ctx, &step)
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

	if err := e.executionRepo.UpdateStep(ctx, &step); err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to update step: %w", err)
	}

	if !output.Success {
		return step, fmt.Errorf("node execution failed: %s", output.Error)
	}

	return step, nil
}

// ExecuteStep executes a single step (for step-by-step execution)
func (e *Engine) ExecuteStep(ctx context.Context, executionID uint, stepID uint) (models.ExecutionStep, error) {
	// Get the step
	step, err := e.getStepByID(ctx, stepID)
	if err != nil {
		return models.ExecutionStep{}, err
	}

	if step.ExecutionID != executionID {
		return models.ExecutionStep{}, fmt.Errorf("step does not belong to execution")
	}

	// Get the execution to retrieve workflow
	execution, err := e.executionRepo.FindByID(ctx, executionID)
	if err != nil {
		return models.ExecutionStep{}, err
	}

	// Parse workflow to get node configuration
	var workflowNodes models.WorkflowNodes
	if err := json.Unmarshal(execution.Workflow.WorkflowJSON, &workflowNodes); err != nil {
		return models.ExecutionStep{}, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	// Find the node for this step
	var nodeConfig *models.NodeConfig
	for _, node := range workflowNodes.Nodes {
		if node.ID == step.NodeID {
			nodeConfig = &node
			break
		}
	}

	if nodeConfig == nil {
		return models.ExecutionStep{}, fmt.Errorf("node configuration not found for step")
	}

	// Get input data (use previous step's output or execution input)
	var inputData map[string]interface{}
	if len(step.InputData) > 0 {
		json.Unmarshal(step.InputData, &inputData)
	}

	// Execute the node
	executedStep, err := e.executeNode(ctx, executionID, *nodeConfig, inputData, step.ExecutionOrder)
	if err != nil {
		return models.ExecutionStep{}, err
	}

	return executedStep, nil
}

// getStepByID is a helper to get a step by ID
func (e *Engine) getStepByID(ctx context.Context, stepID uint) (models.ExecutionStep, error) {
	return e.executionRepo.FindStepByID(ctx, stepID)
}
