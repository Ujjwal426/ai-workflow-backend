package router

import (
	"ai-workflow-builder/internal/config"
	"ai-workflow-builder/internal/handlers"
	httpHandlers "ai-workflow-builder/internal/http/handlers"
	"ai-workflow-builder/internal/service"
	"ai-workflow-builder/internal/websocket"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Dependencies struct {
	Config           config.Config
	WorkflowService  service.WorkflowService
	ExecutionService service.ExecutionService
	WebSocketHandler *websocket.Handler
	AIHandler        *handlers.AIHandler
	ExecutionHandler *handlers.ExecutionHandler
}

func New(deps Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      deps.Config.App.Name,
		ReadTimeout:  deps.Config.Server.ReadTimeout,
		WriteTimeout: deps.Config.Server.WriteTimeout,
		IdleTimeout:  deps.Config.Server.IdleTimeout,
		ErrorHandler: errorHandler,
	})

	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: deps.Config.CORS.AllowOrigins,
		AllowMethods: deps.Config.CORS.AllowMethods,
		AllowHeaders: deps.Config.CORS.AllowHeaders,
	}))

	if !deps.Config.IsProduction() {
		app.Use(logger.New())
	}

	registerRoutes(app, deps)

	return app
}

func registerRoutes(app *fiber.App, deps Dependencies) {
	healthHandler := httpHandlers.NewHealthHandler(deps.Config)
	workflowHandler := httpHandlers.NewWorkflowHandler(deps.WorkflowService)

	api := app.Group("/api")
	api.Get("/health", healthHandler.Check)

	// Workflow routes
	workflows := api.Group("/workflows")
	workflows.Post("", workflowHandler.Create)
	workflows.Post("/validate", workflowHandler.Validate)
	workflows.Get("", workflowHandler.List)
	workflows.Get("/:id", workflowHandler.Get)
	workflows.Put("/:id", workflowHandler.Update)
	workflows.Delete("/:id", workflowHandler.Delete)

	// Execution routes (using new handler)
	executions := api.Group("/executions")
	if deps.ExecutionHandler != nil {
		executions.Post("", deps.ExecutionHandler.ExecuteWorkflow)
		executions.Get("/:id", deps.ExecutionHandler.GetExecution)
		executions.Post("/:id/cancel", deps.ExecutionHandler.CancelExecution)
		executions.Get("/:id/steps", deps.ExecutionHandler.GetExecutionSteps)
		executions.Post("/:id/steps/:stepId/execute", deps.ExecutionHandler.ExecuteStep)
		workflows.Get("/:workflowId/executions", deps.ExecutionHandler.ListExecutions)
	}

	// AI workflow generation routes
	if deps.AIHandler != nil {
		ai := api.Group("/ai")
		ai.Post("/generate-workflow", deps.AIHandler.GenerateWorkflow)
	}

	// WebSocket routes
	if deps.WebSocketHandler != nil {
		app.Get("/ws/executions/:workflowId", deps.WebSocketHandler.HandleWebSocket)
	}
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if fiberErr, ok := err.(*fiber.Error); ok {
		code = fiberErr.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"error": err.Error(),
	})
}
