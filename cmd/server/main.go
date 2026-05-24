package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-workflow-builder/internal/ai"
	"ai-workflow-builder/internal/config"
	"ai-workflow-builder/internal/database"
	"ai-workflow-builder/internal/execution"
	"ai-workflow-builder/internal/executor/nodes"
	"ai-workflow-builder/internal/handlers"
	"ai-workflow-builder/internal/http/router"
	"ai-workflow-builder/internal/models"
	"ai-workflow-builder/internal/repository"
	"ai-workflow-builder/internal/service"
	"ai-workflow-builder/internal/websocket"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file - try multiple locations
	envFiles := []string{".env", "../.env", "../../.env"}
	var loadErr error
	for _, envFile := range envFiles {
		loadErr = godotenv.Load(envFile)
		if loadErr == nil {
			log.Printf("Loaded .env file from: %s", envFile)
			break
		}
	}

	if loadErr != nil {
		log.Printf("Warning: Error loading .env file: %v", loadErr)
		log.Println("You may need to set OPENAI_API_KEY environment variable manually")
	}

	// Debug: Check if OPENAI_API_KEY is set
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		log.Printf("✓ OPENAI_API_KEY is set (length: %d)", len(apiKey))
	} else {
		log.Println("✗ OPENAI_API_KEY is not set")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(&models.Workflow{}, &models.WorkflowExecution{}, &models.ExecutionStep{}); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Fix foreign key constraint for cascade delete (temporary fix until proper migration)
	db.Exec("ALTER TABLE workflow_executions DROP CONSTRAINT IF EXISTS fk_workflows_workflow_executions")
	db.Exec("ALTER TABLE workflow_executions ADD CONSTRAINT fk_workflows_workflow_executions FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE")

	// Initialize repositories
	workflowRepo := repository.NewWorkflowRepository(db)
	executionRepo := repository.NewExecutionRepository(db)

	// Initialize services
	workflowService := service.NewWorkflowService(workflowRepo)

	// Initialize node registry
	nodeRegistry := nodes.NewNodeRegistry()

	// Initialize websocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize execution service with websocket support
	executionService := execution.NewService(executionRepo, workflowRepo, nodeRegistry, wsHub)

	// Initialize AI service
	aiService, err := ai.NewService()
	if err != nil {
		log.Printf("Warning: Failed to initialize AI service: %v", err)
		log.Println("AI workflow generation will not be available")
		aiService = nil
	}

	// Initialize handlers
	websocketHandler := websocket.NewHandler(wsHub)
	aiHandler := handlers.NewAIHandler(aiService)
	executionHandler := handlers.NewExecutionHandler(workflowRepo, executionRepo, executionService)

	app := router.New(router.Dependencies{
		Config:           cfg,
		WorkflowService:  workflowService,
		ExecutionService: executionService,
		WebSocketHandler: websocketHandler,
		AIHandler:        aiHandler,
		ExecutionHandler: executionHandler,
	})

	go func() {
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Fatalf("failed to shutdown server cleanly: %v", err)
	}
}
