package response

import "github.com/gofiber/fiber/v2"

type ErrorBody struct {
	Error string `json:"error"`
}

func JSON(c *fiber.Ctx, status int, payload any) error {
	return c.Status(status).JSON(payload)
}

func Error(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(ErrorBody{Error: message})
}
