package handlers

import (
	"ai-workflow-builder/internal/ai"
	"ai-workflow-builder/internal/http/response"

	"github.com/gofiber/fiber/v2"
)

// AIHandler handles AI-powered workflow generation requests
type AIHandler struct {
	aiService *ai.Service
}

// NewAIHandler creates a new AI handler
func NewAIHandler(aiService *ai.Service) *AIHandler {
	return &AIHandler{
		aiService: aiService,
	}
}

// GenerateWorkflow generates a workflow from a user prompt
func (h *AIHandler) GenerateWorkflow(c *fiber.Ctx) error {
	var request ai.GenerateWorkflowRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if request.Prompt == "" {
		return response.Error(c, fiber.StatusBadRequest, "prompt is required")
	}

	workflow, err := h.aiService.GenerateWorkflow(c.UserContext(), request)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.JSON(c, fiber.StatusOK, workflow)
}