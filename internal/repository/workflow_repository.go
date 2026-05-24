package repository

import (
	"context"

	"ai-workflow-builder/internal/models"

	"gorm.io/gorm"
)

type WorkflowRepository interface {
	Create(ctx context.Context, workflow *models.Workflow) error
	FindAll(ctx context.Context) ([]models.Workflow, error)
	FindByID(ctx context.Context, id uint) (models.Workflow, error)
	Update(ctx context.Context, workflow *models.Workflow) error
	Delete(ctx context.Context, id uint) error
}

type workflowRepository struct {
	db *gorm.DB
}

func NewWorkflowRepository(db *gorm.DB) WorkflowRepository {
	return &workflowRepository{db: db}
}

func (r *workflowRepository) Create(ctx context.Context, workflow *models.Workflow) error {
	return r.db.WithContext(ctx).Create(workflow).Error
}

func (r *workflowRepository) FindAll(ctx context.Context) ([]models.Workflow, error) {
	var workflows []models.Workflow
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&workflows).Error
	return workflows, err
}

func (r *workflowRepository) FindByID(ctx context.Context, id uint) (models.Workflow, error) {
	var workflow models.Workflow
	err := r.db.WithContext(ctx).First(&workflow, id).Error
	return workflow, err
}

func (r *workflowRepository) Update(ctx context.Context, workflow *models.Workflow) error {
	return r.db.WithContext(ctx).Save(workflow).Error
}

func (r *workflowRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Workflow{}, id).Error
}
