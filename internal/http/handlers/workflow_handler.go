package handlers

import (
	"errors"
	"fmt"
	"strconv"

	"ai-workflow-builder/internal/http/response"
	"ai-workflow-builder/internal/service"

	"github.com/gofiber/fiber/v2"
)

type WorkflowHandler struct {
	service service.WorkflowService
}

func NewWorkflowHandler(service service.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{service: service}
}

func (h *WorkflowHandler) Create(c *fiber.Ctx) error {
	var input service.WorkflowInput
	if err := c.BodyParser(&input); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	workflow, err := h.service.Create(c.UserContext(), input)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusCreated, workflow)
}

func (h *WorkflowHandler) List(c *fiber.Ctx) error {
	workflows, err := h.service.List(c.UserContext())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "failed to list workflows")
	}

	return response.JSON(c, fiber.StatusOK, workflows)
}

func (h *WorkflowHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid workflow id")
	}

	workflow, err := h.service.Get(c.UserContext(), id)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusOK, workflow)
}

func (h *WorkflowHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid workflow id")
	}

	var input service.WorkflowInput
	if err := c.BodyParser(&input); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	workflow, err := h.service.Update(c.UserContext(), id, input)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return response.JSON(c, fiber.StatusOK, workflow)
}

func (h *WorkflowHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid workflow id")
	}

	if err := h.service.Delete(c.UserContext(), id); err != nil {
		return h.handleServiceError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *WorkflowHandler) Validate(c *fiber.Ctx) error {
	var input service.WorkflowInput
	if err := c.BodyParser(&input); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	validation, err := h.service.ValidateWorkflow(c.UserContext(), input.WorkflowJSON)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "validation failed")
	}

	return response.JSON(c, fiber.StatusOK, validation)
}

func (h *WorkflowHandler) handleServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidWorkflow):
		return response.Error(c, fiber.StatusBadRequest, "name is required")
	case errors.Is(err, service.ErrWorkflowNotFound):
		return response.Error(c, fiber.StatusNotFound, "workflow not found")
	default:
		return response.Error(c, fiber.StatusInternalServerError, "internal server error")
	}
}

func parseID(c *fiber.Ctx) (uint, error) {
	rawID := c.Params("id")
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid id")
	}

	return uint(id), nil
}
