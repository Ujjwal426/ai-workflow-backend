package handlers

import (
	"time"

	"ai-workflow-builder/internal/config"

	"github.com/gofiber/fiber/v2"
)

type HealthHandler struct {
	cfg       config.Config
	startedAt time.Time
}

func NewHealthHandler(cfg config.Config) *HealthHandler {
	return &HealthHandler{
		cfg:       cfg,
		startedAt: time.Now().UTC(),
	}
}

func (h *HealthHandler) Check(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":      "ok",
		"service":     h.cfg.App.Name,
		"environment": h.cfg.App.Environment,
		"startedAt":   h.startedAt,
	})
}
