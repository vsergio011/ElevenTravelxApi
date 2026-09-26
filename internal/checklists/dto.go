package checklists

import (
	"time"

	"github.com/google/uuid"
)

type CreateChecklistTaskRequest struct {
	Title           string   `json:"title"`
	Type            string   `json:"type"`
	Priority        string   `json:"priority"`
	AssignmentMode  string   `json:"assignment_mode"`
	DueDate         *string  `json:"due_date"`
	AssignedUserIDs []string `json:"assigned_user_ids"`
}

type UpdateChecklistTaskRequest struct {
	Title           *string   `json:"title"`
	Type            *string   `json:"type"`
	Priority        *string   `json:"priority"`
	AssignmentMode  *string   `json:"assignment_mode"`
	DueDate         *string   `json:"due_date"`
	AssignedUserIDs *[]string `json:"assigned_user_ids"`
}

type ChecklistTaskResponse struct {
	ID                 uuid.UUID   `json:"id"`
	PlanningID         uuid.UUID   `json:"planning_id"`
	CreatedByUserID    uuid.UUID   `json:"created_by_user_id"`
	Title              string      `json:"title"`
	Type               string      `json:"type"`
	Priority           string      `json:"priority"`
	AssignmentMode     string      `json:"assignment_mode"`
	AssignedUserIDs    []uuid.UUID `json:"assigned_user_ids"`
	CompletedByUserIDs []uuid.UUID `json:"completed_by_user_ids"`
	DueDate            *string     `json:"due_date"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

func NewChecklistTaskResponse(task ChecklistTask) ChecklistTaskResponse {
	var dueDate *string
	if task.DueDate != nil {
		formatted := task.DueDate.Format("2006-01-02")
		dueDate = &formatted
	}

	return ChecklistTaskResponse{
		ID:                 task.ID,
		PlanningID:         task.PlanningID,
		CreatedByUserID:    task.CreatedByUserID,
		Title:              task.Title,
		Type:               task.Type,
		Priority:           task.Priority,
		AssignmentMode:     task.AssignmentMode,
		AssignedUserIDs:    nonNilUUIDs(task.AssignedUserIDs),
		CompletedByUserIDs: nonNilUUIDs(task.CompletedByUserIDs),
		DueDate:            dueDate,
		CreatedAt:          task.CreatedAt,
		UpdatedAt:          task.UpdatedAt,
	}
}

func nonNilUUIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}

	return ids
}
