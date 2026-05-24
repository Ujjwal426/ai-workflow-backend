package handlers

import (
	"encoding/json"
	"fmt"

	"ai-workflow-builder/internal/execution"
	"ai-workflow-builder/internal/http/response"
	"ai-workflow-builder/internal/repository"
	"ai-workflow-builder/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ExecutionHandler handles workflow execution requests
type ExecutionHandler struct {
	workflowRepo  repository.WorkflowRepository
	executionRepo repository.ExecutionRepository
	executionSvc  *execution.Service
}

// NewExecutionHandler creates a new execution handler
func NewExecutionHandler(
	workflowRepo repository.WorkflowRepository,
	executionRepo repository.ExecutionRepository,
	executionSvc *execution.Service,
) *ExecutionHandler {
	return &ExecutionHandler{
		workflowRepo:  workflowRepo,
		executionRepo: executionRepo,
		executionSvc:  executionSvc,
	}
}

// ExecuteWorkflow executes a workflow (supports both legacy and new execution service)
func (h *ExecutionHandler) ExecuteWorkflow(c *fiber.Ctx) error {
	// Try to parse as direct execution first (with nodes/edges)
	var rawBody map[string]interface{}
	if err := c.BodyParser(&rawBody); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	// Check if this is a direct execution (has nodes array)
	if _, hasNodes := rawBody["nodes"]; hasNodes {
		// Handle workflowId that might be a string
		workflowID := uuid.Nil
		if wid, ok := rawBody["workflowId"]; ok {
			switch v := wid.(type) {
			case string:
				if parsed, err := uuid.Parse(v); err == nil {
					workflowID = parsed
				}
			}
		}

		// Extract inputData
		inputData := make(map[string]interface{})
		if data, ok := rawBody["inputData"].(map[string]interface{}); ok {
			inputData = data
		}

		// Parse nodes and edges
		directInput := service.DirectExecutionInput{
			WorkflowID: workflowID,
			InputData:  inputData,
		}

		// Marshal and unmarshal to properly parse nodes/edges
		bodyBytes, _ := json.Marshal(rawBody)
		json.Unmarshal(bodyBytes, &directInput)

		execution, err := h.executionSvc.ExecuteWorkflowDirect(c.UserContext(), directInput)
		if err != nil {
			return h.handleServiceError(c, err)
		}
		return response.JSON(c, fiber.StatusCreated, execution)
	}

	// Fall back to regular execution by workflow ID
	var input service.ExecutionInput
	if err := c.BodyParser(&input); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	// Use the new execution service with websocket support
	execution, err := h.executionSvc.ExecuteWorkflow(c.UserContext(), input.WorkflowID, input.InputData)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusCreated, execution)
}

// GetExecution retrieves a specific execution by ID
func (h *ExecutionHandler) GetExecution(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid execution id")
	}

	execution, err := h.executionRepo.FindByID(c.UserContext(), id)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "execution not found")
	}

	return response.JSON(c, fiber.StatusOK, execution)
}

// ListExecutions retrieves all executions for a specific workflow
func (h *ExecutionHandler) ListExecutions(c *fiber.Ctx) error {
	workflowIDParam := c.Params("workflowId")
	workflowID, err := uuid.Parse(workflowIDParam)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid workflow id")
	}

	executions, err := h.executionRepo.FindByWorkflowID(c.UserContext(), workflowID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "failed to list executions")
	}

	return response.JSON(c, fiber.StatusOK, executions)
}

// GetExecutionSteps retrieves all steps for a specific execution
func (h *ExecutionHandler) GetExecutionSteps(c *fiber.Ctx) error {
	executionID, err := parseID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid execution id")
	}

	steps, err := h.executionRepo.FindStepsByExecutionID(c.UserContext(), executionID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "failed to get steps")
	}

	return response.JSON(c, fiber.StatusOK, steps)
}

// ExecuteStep executes a single step (for step-by-step execution)
func (h *ExecutionHandler) ExecuteStep(c *fiber.Ctx) error {
	executionID, err := parseID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid execution id")
	}

	stepIDParam := c.Params("stepId")
	stepID, err := uuid.Parse(stepIDParam)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid step id")
	}

	step, err := h.executionSvc.ExecuteStep(c.UserContext(), executionID, stepID)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusOK, step)
}

// CancelExecution cancels a running execution
func (h *ExecutionHandler) CancelExecution(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid execution id")
	}

	if err := h.executionSvc.CancelExecution(c.UserContext(), id); err != nil {
		return h.handleServiceError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ExecutionHandler) handleServiceError(c *fiber.Ctx, err error) error {
	switch {
	case err != nil && err.Error() == "invalid execution request":
		return response.Error(c, fiber.StatusBadRequest, "invalid execution request")
	case err != nil && err.Error() == "workflow not found":
		return response.Error(c, fiber.StatusNotFound, "workflow not found")
	case err != nil && err.Error() == "execution not found":
		return response.Error(c, fiber.StatusNotFound, "execution not found")
	default:
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("internal server error: %v", err))
	}
}

func parseID(c *fiber.Ctx) (uuid.UUID, error) {
	rawID := c.Params("id")
	id, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid id")
	}
	return id, nil
}
