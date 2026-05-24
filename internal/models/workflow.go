package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Workflow struct {
	ID                 uuid.UUID           `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name               string              `json:"name" gorm:"type:varchar(160);not null"`
	Description        string              `json:"description" gorm:"type:text"`
	WorkflowJSON       datatypes.JSON      `json:"workflowJson" gorm:"type:jsonb;not null;default:'{}'"`
	WorkflowExecutions []WorkflowExecution `json:"workflowExecutions" gorm:"foreignKey:WorkflowID;constraint:OnDelete:CASCADE"`
	CreatedAt          time.Time           `json:"createdAt"`
	UpdatedAt          time.Time           `json:"updatedAt"`
}

func (w *Workflow) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}
