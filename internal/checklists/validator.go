package checklists

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

func isValidPriority(priority string) bool {
	switch priority {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}

func isValidAssignmentMode(mode string) bool {
	switch mode {
	case AssignmentModeGroup, AssignmentModeIndividual:
		return true
	default:
		return false
	}
}

func parseOptionalDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

func parseUserIDs(rawIDs []string) ([]uuid.UUID, []response.ErrorDetail) {
	details := make([]response.ErrorDetail, 0)
	ids := make([]uuid.UUID, 0, len(rawIDs))

	for _, rawID := range rawIDs {
		parsedID, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			details = append(details, response.ErrorDetail{Field: "assigned_user_ids", Message: "assigned_user_ids must contain valid UUIDs"})
			continue
		}
		ids = append(ids, parsedID)
	}

	return ids, details
}

func validateCreateChecklistTaskRequest(request CreateChecklistTaskRequest) (CreateChecklistTaskInput, error) {
	details := make([]response.ErrorDetail, 0)

	title := strings.TrimSpace(request.Title)
	if len(title) < 3 || len(title) > 80 {
		details = append(details, response.ErrorDetail{Field: "title", Message: "title must be between 3 and 80 characters"})
	}

	taskType := strings.ToLower(strings.TrimSpace(request.Type))
	if len(taskType) < 2 || len(taskType) > 30 {
		details = append(details, response.ErrorDetail{Field: "type", Message: "type must be between 2 and 30 characters"})
	}

	priority := strings.TrimSpace(request.Priority)
	if !isValidPriority(priority) {
		details = append(details, response.ErrorDetail{Field: "priority", Message: "priority must be low, medium or high"})
	}

	assignmentMode := strings.TrimSpace(request.AssignmentMode)
	if !isValidAssignmentMode(assignmentMode) {
		details = append(details, response.ErrorDetail{Field: "assignment_mode", Message: "assignment_mode must be group or individual"})
	}

	dueDate, err := parseOptionalDate(request.DueDate)
	if err != nil {
		details = append(details, response.ErrorDetail{Field: "due_date", Message: "due_date must have format YYYY-MM-DD"})
	}

	assignedUserIDs, idDetails := parseUserIDs(request.AssignedUserIDs)
	details = append(details, idDetails...)

	if len(details) > 0 {
		return CreateChecklistTaskInput{}, ValidationError{Details: details}
	}

	return CreateChecklistTaskInput{
		Title:           title,
		Type:            taskType,
		Priority:        priority,
		AssignmentMode:  assignmentMode,
		AssignedUserIDs: assignedUserIDs,
		DueDate:         dueDate,
	}, nil
}

func validateUpdateChecklistTaskRequest(request UpdateChecklistTaskRequest) (UpdateChecklistTaskInput, error) {
	details := make([]response.ErrorDetail, 0)
	input := UpdateChecklistTaskInput{}

	if request.Title != nil {
		title := strings.TrimSpace(*request.Title)
		if len(title) < 3 || len(title) > 80 {
			details = append(details, response.ErrorDetail{Field: "title", Message: "title must be between 3 and 80 characters"})
		} else {
			input.Title = &title
		}
	}

	if request.Type != nil {
		taskType := strings.ToLower(strings.TrimSpace(*request.Type))
		if len(taskType) < 2 || len(taskType) > 30 {
			details = append(details, response.ErrorDetail{Field: "type", Message: "type must be between 2 and 30 characters"})
		} else {
			input.Type = &taskType
		}
	}

	if request.Priority != nil {
		priority := strings.TrimSpace(*request.Priority)
		if !isValidPriority(priority) {
			details = append(details, response.ErrorDetail{Field: "priority", Message: "priority must be low, medium or high"})
		} else {
			input.Priority = &priority
		}
	}

	if request.AssignmentMode != nil {
		assignmentMode := strings.TrimSpace(*request.AssignmentMode)
		if !isValidAssignmentMode(assignmentMode) {
			details = append(details, response.ErrorDetail{Field: "assignment_mode", Message: "assignment_mode must be group or individual"})
		} else {
			input.AssignmentMode = &assignmentMode
		}
	}

	if request.DueDate != nil {
		if strings.TrimSpace(*request.DueDate) == "" {
			input.ClearDueDate = true
		} else {
			dueDate, err := parseOptionalDate(request.DueDate)
			if err != nil {
				details = append(details, response.ErrorDetail{Field: "due_date", Message: "due_date must have format YYYY-MM-DD"})
			} else {
				input.DueDate = dueDate
			}
		}
	}

	if request.AssignedUserIDs != nil {
		assignedUserIDs, idDetails := parseUserIDs(*request.AssignedUserIDs)
		details = append(details, idDetails...)
		input.AssignedUserIDs = &assignedUserIDs
	}

	if len(details) > 0 {
		return UpdateChecklistTaskInput{}, ValidationError{Details: details}
	}

	return input, nil
}
