package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"ai-workflow-builder/internal/executor"
	"ai-workflow-builder/internal/executor/nodes"
	"ai-workflow-builder/internal/models"
	"ai-workflow-builder/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrExecutionNotFound = errors.New("execution not found")
	ErrInvalidExecution  = errors.New("invalid execution payload")
)

type ExecutionService interface {
	ExecuteWorkflow(ctx context.Context, workflowID uint, inputData map[string]interface{}) (models.WorkflowExecution, error)
	ExecuteWorkflowDirect(ctx context.Context, executionInput DirectExecutionInput) (models.WorkflowExecution, error)
	GetExecution(ctx context.Context, id uint) (models.WorkflowExecution, error)
	ListExecutions(ctx context.Context, workflowID uint) ([]models.WorkflowExecution, error)
	ExecuteStep(ctx context.Context, executionID uint, stepID uint) (models.ExecutionStep, error)
	GetExecutionSteps(ctx context.Context, executionID uint) ([]models.ExecutionStep, error)
	CancelExecution(ctx context.Context, id uint) error
}

type executionService struct {
	workflowRepo  repository.WorkflowRepository
	executionRepo repository.ExecutionRepository
	engine        *executor.Engine
}

type ExecutionInput struct {
	WorkflowID uint                   `json:"workflowId"`
	InputData  map[string]interface{} `json:"inputData"`
}

type DirectExecutionInput struct {
	WorkflowID uint                   `json:"workflowId"`
	Nodes      []models.NodeConfig    `json:"nodes"`
	Edges      []models.EdgeConfig    `json:"edges"`
	InputData  map[string]interface{} `json:"inputData"`
}

func NewExecutionService(
	workflowRepo repository.WorkflowRepository,
	executionRepo repository.ExecutionRepository,
	nodeRegistry *nodes.NodeRegistry,
) ExecutionService {
	return &executionService{
		workflowRepo:  workflowRepo,
		executionRepo: executionRepo,
		engine:        executor.NewEngine(executionRepo, nodeRegistry),
	}
}

func (s *executionService) ExecuteWorkflow(ctx context.Context, workflowID uint, inputData map[string]interface{}) (models.WorkflowExecution, error) {
	// Validate input
	if workflowID == 0 {
		return models.WorkflowExecution{}, ErrInvalidExecution
	}

	if inputData == nil {
		inputData = make(map[string]interface{})
	}

	// Get workflow
	workflow, err := s.workflowRepo.FindByID(ctx, workflowID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.WorkflowExecution{}, ErrWorkflowNotFound
	}
	if err != nil {
		return models.WorkflowExecution{}, err
	}

	// Execute workflow using engine
	execution, err := s.engine.ExecuteWorkflow(ctx, workflow, inputData)
	if err != nil {
		return models.WorkflowExecution{}, err
	}

	return execution, nil
}

func (s *executionService) ExecuteWorkflowDirect(ctx context.Context, input DirectExecutionInput) (models.WorkflowExecution, error) {
	// Validate input
	if len(input.Nodes) == 0 {
		return models.WorkflowExecution{}, ErrInvalidExecution
	}

	if input.InputData == nil {
		input.InputData = make(map[string]interface{})
	}

	// Create a temporary workflow object for execution
	workflow := models.Workflow{
		ID: input.WorkflowID,
		// We'll construct the workflow JSON from the provided nodes and edges
	}

	// Convert nodes and edges to workflow JSON
	workflowNodes := models.WorkflowNodes{
		Nodes: input.Nodes,
		Edges: input.Edges,
	}

	workflowJSON, err := json.Marshal(workflowNodes)
	if err != nil {
		return models.WorkflowExecution{}, fmt.Errorf("failed to marshal workflow structure: %w", err)
	}

	workflow.WorkflowJSON = workflowJSON

	// Execute workflow using engine
	execution, err := s.engine.ExecuteWorkflow(ctx, workflow, input.InputData)
	if err != nil {
		return models.WorkflowExecution{}, err
	}

	return execution, nil
}

func (s *executionService) GetExecution(ctx context.Context, id uint) (models.WorkflowExecution, error) {
	execution, err := s.executionRepo.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.WorkflowExecution{}, ErrExecutionNotFound
	}

	return execution, err
}

func (s *executionService) ListExecutions(ctx context.Context, workflowID uint) ([]models.WorkflowExecution, error) {
	if workflowID == 0 {
		return nil, ErrInvalidExecution
	}

	return s.executionRepo.FindByWorkflowID(ctx, workflowID)
}

func (s *executionService) ExecuteStep(ctx context.Context, executionID uint, stepID uint) (models.ExecutionStep, error) {
	if executionID == 0 || stepID == 0 {
		return models.ExecutionStep{}, ErrInvalidExecution
	}

	// Get execution to ensure it exists
	_, err := s.GetExecution(ctx, executionID)
	if err != nil {
		return models.ExecutionStep{}, err
	}

	// Execute step using engine
	step, err := s.engine.ExecuteStep(ctx, executionID, stepID)
	if err != nil {
		return models.ExecutionStep{}, err
	}

	return step, nil
}

func (s *executionService) GetExecutionSteps(ctx context.Context, executionID uint) ([]models.ExecutionStep, error) {
	if executionID == 0 {
		return nil, ErrInvalidExecution
	}

	return s.executionRepo.FindStepsByExecutionID(ctx, executionID)
}

func (s *executionService) CancelExecution(ctx context.Context, id uint) error {
	if id == 0 {
		return ErrInvalidExecution
	}

	execution, err := s.GetExecution(ctx, id)
	if err != nil {
		return err
	}

	// Only allow cancellation of running or pending executions
	if execution.Status != models.ExecutionStatusRunning && execution.Status != models.ExecutionStatusPending {
		return errors.New("can only cancel running or pending executions")
	}

	execution.Status = models.ExecutionStatusCancelled
	return s.executionRepo.UpdateExecution(ctx, &execution)
}
