package models

import (
	"time"

	"gorm.io/datatypes"
)

type Workflow struct {
	ID                 uint                `json:"id" gorm:"primaryKey"`
	Name               string              `json:"name" gorm:"type:varchar(160);not null"`
	Description        string              `json:"description" gorm:"type:text"`
	WorkflowJSON       datatypes.JSON      `json:"workflowJson" gorm:"type:jsonb;not null;default:'{}'"`
	WorkflowExecutions []WorkflowExecution `json:"workflowExecutions" gorm:"foreignKey:WorkflowID;constraint:OnDelete:CASCADE"`
	CreatedAt          time.Time           `json:"createdAt"`
	UpdatedAt          time.Time           `json:"updatedAt"`
}
