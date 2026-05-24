package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ai-workflow-builder/internal/models"
	"ai-workflow-builder/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrInvalidWorkflow  = errors.New("invalid workflow payload")
)

type WorkflowService interface {
	Create(ctx context.Context, input WorkflowInput) (models.Workflow, error)
	List(ctx context.Context) ([]models.Workflow, error)
	Get(ctx context.Context, id uint) (models.Workflow, error)
	Update(ctx context.Context, id uint, input WorkflowInput) (models.Workflow, error)
	Delete(ctx context.Context, id uint) error
	ValidateWorkflow(ctx context.Context, workflowJSON datatypes.JSON) (models.WorkflowValidation, error)
}

type WorkflowInput struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	WorkflowJSON datatypes.JSON `json:"workflowJson"`
}

type workflowService struct {
	repo repository.WorkflowRepository
}

func NewWorkflowService(repo repository.WorkflowRepository) WorkflowService {
	return &workflowService{repo: repo}
}

func (s *workflowService) Create(ctx context.Context, input WorkflowInput) (models.Workflow, error) {
	if err := validateWorkflowInput(input); err != nil {
		return models.Workflow{}, err
	}

	workflow := models.Workflow{
		Name:         strings.TrimSpace(input.Name),
		Description:  strings.TrimSpace(input.Description),
		WorkflowJSON: normalizeWorkflowJSON(input.WorkflowJSON),
	}

	if err := s.repo.Create(ctx, &workflow); err != nil {
		return models.Workflow{}, err
	}

	return workflow, nil
}

func (s *workflowService) List(ctx context.Context) ([]models.Workflow, error) {
	return s.repo.FindAll(ctx)
}

func (s *workflowService) Get(ctx context.Context, id uint) (models.Workflow, error) {
	workflow, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Workflow{}, ErrWorkflowNotFound
	}

	return workflow, err
}

func (s *workflowService) Update(ctx context.Context, id uint, input WorkflowInput) (models.Workflow, error) {
	if err := validateWorkflowInput(input); err != nil {
		return models.Workflow{}, err
	}

	workflow, err := s.Get(ctx, id)
	if err != nil {
		return models.Workflow{}, err
	}

	workflow.Name = strings.TrimSpace(input.Name)
	workflow.Description = strings.TrimSpace(input.Description)
	workflow.WorkflowJSON = normalizeWorkflowJSON(input.WorkflowJSON)

	if err := s.repo.Update(ctx, &workflow); err != nil {
		return models.Workflow{}, err
	}

	return workflow, nil
}

func (s *workflowService) Delete(ctx context.Context, id uint) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}

	return s.repo.Delete(ctx, id)
}

func validateWorkflowInput(input WorkflowInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return ErrInvalidWorkflow
	}

	if len(input.WorkflowJSON) > 0 && !json.Valid(input.WorkflowJSON) {
		return ErrInvalidWorkflow
	}

	return nil
}

func normalizeWorkflowJSON(value datatypes.JSON) datatypes.JSON {
	if len(value) == 0 {
		return datatypes.JSON([]byte("{}"))
	}

	return value
}

func (s *workflowService) ValidateWorkflow(ctx context.Context, workflowJSON datatypes.JSON) (models.WorkflowValidation, error) {
	validation := models.WorkflowValidation{
		IsValid:  true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Parse workflow JSON
	var workflowNodes models.WorkflowNodes
	if err := json.Unmarshal(workflowJSON, &workflowNodes); err != nil {
		validation.IsValid = false
		validation.Errors = append(validation.Errors, "Invalid JSON format")
		return validation, nil
	}

	// Check if nodes exist
	if len(workflowNodes.Nodes) == 0 {
		validation.IsValid = false
		validation.Errors = append(validation.Errors, "Workflow must have at least one node")
		return validation, nil
	}

	// Check for start node
	hasStart := false
	hasEnd := false
	for _, node := range workflowNodes.Nodes {
		// Use GetConfig() to handle both config formats
		config := node.GetConfig()
		node.Config = config // Update config for validation

		if node.Type == models.NodeTypeStart || node.Type == models.NodeTypeStartNode {
			hasStart = true
		}
		if node.Type == models.NodeTypeEnd || node.Type == models.NodeTypeEndNode {
			hasEnd = true
		}
	}

	if !hasStart {
		validation.IsValid = false
		validation.Errors = append(validation.Errors, "Workflow must have a Start node")
	}

	if !hasEnd {
		validation.Warnings = append(validation.Warnings, "Workflow should have an End node for proper completion")
	}

	// Check for valid edge connections
	nodeIds := make(map[string]bool)
	for _, node := range workflowNodes.Nodes {
		nodeIds[node.ID] = true
	}

	for _, edge := range workflowNodes.Edges {
		if !nodeIds[edge.Source] {
			validation.IsValid = false
			validation.Errors = append(validation.Errors, fmt.Sprintf("Edge source node '%s' does not exist", edge.Source))
		}
		if !nodeIds[edge.Target] {
			validation.IsValid = false
			validation.Errors = append(validation.Errors, fmt.Sprintf("Edge target node '%s' does not exist", edge.Target))
		}
	}

	// Check for orphaned nodes (nodes without connections)
	if len(workflowNodes.Edges) > 0 {
		connectedNodes := make(map[string]bool)
		for _, edge := range workflowNodes.Edges {
			connectedNodes[edge.Source] = true
			connectedNodes[edge.Target] = true
		}

		for _, node := range workflowNodes.Nodes {
			if !connectedNodes[node.ID] && node.Type != models.NodeTypeStart && node.Type != models.NodeTypeStartNode {
				validation.Warnings = append(validation.Warnings, fmt.Sprintf("Node '%s' is not connected to the workflow", node.ID))
			}
		}
	}

	// Check for multiple start nodes
	startCount := 0
	for _, node := range workflowNodes.Nodes {
		if node.Type == models.NodeTypeStart || node.Type == models.NodeTypeStartNode {
			startCount++
		}
	}
	if startCount > 1 {
		validation.IsValid = false
		validation.Errors = append(validation.Errors, "Workflow cannot have multiple Start nodes")
	}

	return validation, nil
}
