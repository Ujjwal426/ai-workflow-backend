package handlers

import (
	"encoding/json"
	"errors"

	"ai-workflow-builder/internal/http/response"
	"ai-workflow-builder/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ExecutionHandler struct {
	service service.ExecutionService
}

func NewExecutionHandler(service service.ExecutionService) *ExecutionHandler {
	return &ExecutionHandler{service: service}
}

// ExecuteWorkflow creates and executes a new workflow execution
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

		execution, err := h.service.ExecuteWorkflowDirect(c.UserContext(), directInput)
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

	execution, err := h.service.ExecuteWorkflow(c.UserContext(), input.WorkflowID, input.InputData)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusCreated, execution)
}

// GetExecution retrieves a specific execution by ID
func (h *ExecutionHandler) GetExecution(c *fiber.Ctx) error {
	id, err := parseUUID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid execution id")
	}

	execution, err := h.service.GetExecution(c.UserContext(), id)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusOK, execution)
}

// ListExecutions retrieves all executions for a specific workflow
func (h *ExecutionHandler) ListExecutions(c *fiber.Ctx) error {
	// Try both parameter names that might be used
	workflowIDParam := c.Params("workflowId")
	if workflowIDParam == "" {
		workflowIDParam = c.Params("id")
	}

	workflowID, err := uuid.Parse(workflowIDParam)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid workflow id")
	}

	executions, err := h.service.ListExecutions(c.UserContext(), workflowID)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusOK, executions)
}

// GetExecutionSteps retrieves all steps for a specific execution
func (h *ExecutionHandler) GetExecutionSteps(c *fiber.Ctx) error {
	executionID, err := parseUUID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid execution id")
	}

	steps, err := h.service.GetExecutionSteps(c.UserContext(), executionID)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusOK, steps)
}

// ExecuteStep executes a single step (for step-by-step execution)
func (h *ExecutionHandler) ExecuteStep(c *fiber.Ctx) error {
	executionID, err := parseUUID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid execution id")
	}

	stepIDParam := c.Params("stepId")
	stepID, err := uuid.Parse(stepIDParam)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid step id")
	}

	step, err := h.service.ExecuteStep(c.UserContext(), executionID, stepID)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusOK, step)
}

// CancelExecution cancels a running execution
func (h *ExecutionHandler) CancelExecution(c *fiber.Ctx) error {
	id, err := parseUUID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid execution id")
	}

	if err := h.service.CancelExecution(c.UserContext(), id); err != nil {
		return h.handleServiceError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ExecutionHandler) handleServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidExecution):
		return response.Error(c, fiber.StatusBadRequest, "invalid execution request")
	case errors.Is(err, service.ErrWorkflowNotFound):
		return response.Error(c, fiber.StatusNotFound, "workflow not found")
	case errors.Is(err, service.ErrExecutionNotFound):
		return response.Error(c, fiber.StatusNotFound, "execution not found")
	default:
		return response.Error(c, fiber.StatusInternalServerError, "internal server error")
	}
}
