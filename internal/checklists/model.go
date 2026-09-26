package checklists

import (
	"time"

	"github.com/google/uuid"
)

const (
	AssignmentModeGroup      = "group"
	AssignmentModeIndividual = "individual"
)

const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

type ChecklistTask struct {
	ID                 uuid.UUID
	PlanningID         uuid.UUID
	CreatedByUserID    uuid.UUID
	Title              string
	Type               string
	Priority           string
	AssignmentMode     string
	AssignedUserIDs    []uuid.UUID
	CompletedByUserIDs []uuid.UUID
	DueDate            *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ChecklistTaskFilters struct {
	Type *string
}

type CreateChecklistTaskInput struct {
	Title           string
	Type            string
	Priority        string
	AssignmentMode  string
	AssignedUserIDs []uuid.UUID
	DueDate         *time.Time
}

type UpdateChecklistTaskInput struct {
	Title           *string
	Type            *string
	Priority        *string
	AssignmentMode  *string
	AssignedUserIDs *[]uuid.UUID
	DueDate         *time.Time
	ClearDueDate    bool
}
