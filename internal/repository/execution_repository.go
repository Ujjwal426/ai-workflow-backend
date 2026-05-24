package repository

import (
	"context"

	"ai-workflow-builder/internal/models"

	"gorm.io/gorm"
)

type ExecutionRepository interface {
	CreateExecution(ctx context.Context, execution *models.WorkflowExecution) error
	FindByID(ctx context.Context, id uint) (models.WorkflowExecution, error)
	FindByWorkflowID(ctx context.Context, workflowID uint) ([]models.WorkflowExecution, error)
	UpdateExecution(ctx context.Context, execution *models.WorkflowExecution) error
	DeleteExecution(ctx context.Context, id uint) error
	
	CreateStep(ctx context.Context, step *models.ExecutionStep) error
	FindStepsByExecutionID(ctx context.Context, executionID uint) ([]models.ExecutionStep, error)
	FindStepByID(ctx context.Context, stepID uint) (models.ExecutionStep, error)
	UpdateStep(ctx context.Context, step *models.ExecutionStep) error
	FindPendingSteps(ctx context.Context, executionID uint) ([]models.ExecutionStep, error)
}

type executionRepository struct {
	db *gorm.DB
}

func NewExecutionRepository(db *gorm.DB) ExecutionRepository {
	return &executionRepository{db: db}
}

func (r *executionRepository) CreateExecution(ctx context.Context, execution *models.WorkflowExecution) error {
	return r.db.WithContext(ctx).Create(execution).Error
}

func (r *executionRepository) FindByID(ctx context.Context, id uint) (models.WorkflowExecution, error) {
	var execution models.WorkflowExecution
	err := r.db.WithContext(ctx).
		Preload("ExecutionSteps").
		Preload("Workflow").
		First(&execution, id).Error
	return execution, err
}

func (r *executionRepository) FindByWorkflowID(ctx context.Context, workflowID uint) ([]models.WorkflowExecution, error) {
	var executions []models.WorkflowExecution
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Preload("ExecutionSteps").
		Order("created_at DESC").
		Find(&executions).Error
	return executions, err
}

func (r *executionRepository) UpdateExecution(ctx context.Context, execution *models.WorkflowExecution) error {
	return r.db.WithContext(ctx).Save(execution).Error
}

func (r *executionRepository) DeleteExecution(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.WorkflowExecution{}, id).Error
}

func (r *executionRepository) CreateStep(ctx context.Context, step *models.ExecutionStep) error {
	return r.db.WithContext(ctx).Create(step).Error
}

func (r *executionRepository) FindStepsByExecutionID(ctx context.Context, executionID uint) ([]models.ExecutionStep, error) {
	var steps []models.ExecutionStep
	err := r.db.WithContext(ctx).
		Where("execution_id = ?", executionID).
		Order("execution_order ASC").
		Find(&steps).Error
	return steps, err
}

func (r *executionRepository) FindStepByID(ctx context.Context, stepID uint) (models.ExecutionStep, error) {
	var step models.ExecutionStep
	err := r.db.WithContext(ctx).First(&step, stepID).Error
	return step, err
}

func (r *executionRepository) UpdateStep(ctx context.Context, step *models.ExecutionStep) error {
	return r.db.WithContext(ctx).Save(step).Error
}

func (r *executionRepository) FindPendingSteps(ctx context.Context, executionID uint) ([]models.ExecutionStep, error) {
	var steps []models.ExecutionStep
	err := r.db.WithContext(ctx).
		Where("execution_id = ? AND status = ?", executionID, models.StepStatusPending).
		Order("execution_order ASC").
		Find(&steps).Error
	return steps, err
}