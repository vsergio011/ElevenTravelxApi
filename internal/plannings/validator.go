package plannings

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func validateCreatePlanningRequest(request CreatePlanningRequest) (CreatePlanningInput, error) {
	details := make([]response.ErrorDetail, 0)

	var groupID *uuid.UUID
	groupIDRaw := strings.TrimSpace(request.GroupID)
	if groupIDRaw != "" {
		parsedGroupID, err := uuid.Parse(groupIDRaw)
		if err != nil {
			details = append(details, response.ErrorDetail{Field: "group_id", Message: "group_id must be a valid UUID"})
		} else {
			groupID = &parsedGroupID
		}
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		details = append(details, response.ErrorDetail{Field: "name", Message: "name is required"})
	}

	startsAt, endsAt, dateDetails := validateDateRange(request.StartsAt, request.EndsAt)
	details = append(details, dateDetails...)

	if len(details) > 0 {
		return CreatePlanningInput{}, ValidationError{Details: details}
	}

	return CreatePlanningInput{
		GroupID:         groupID,
		Name:            name,
		Description:     trimOptionalString(request.Description),
		DestinationName: trimOptionalString(request.DestinationName),
		StartsAt:        startsAt,
		EndsAt:          endsAt,
	}, nil
}

func validateUpdatePlanningRequest(request UpdatePlanningRequest) (UpdatePlanningInput, error) {
	details := make([]response.ErrorDetail, 0)

	startsAt, endsAt, dateDetails := validateDateRange(request.StartsAt, request.EndsAt)
	details = append(details, dateDetails...)

	if request.Status != nil {
		status := strings.TrimSpace(*request.Status)
		if !isValidPlanningStatus(status) {
			details = append(details, response.ErrorDetail{Field: "status", Message: fmt.Sprintf("status must be one of %s, %s, %s, %s", StatusDraft, StatusConfirmed, StatusInProgress, StatusFinished)})
		} else {
			request.Status = &status
		}
	}

	if len(details) > 0 {
		return UpdatePlanningInput{}, ValidationError{Details: details}
	}

	name := trimOptionalString(request.Name)
	if name != nil && *name == "" {
		return UpdatePlanningInput{}, ValidationError{Details: []response.ErrorDetail{{Field: "name", Message: "name cannot be empty"}}}
	}

	return UpdatePlanningInput{
		Name:            name,
		Description:     trimOptionalString(request.Description),
		DestinationName: trimOptionalString(request.DestinationName),
		StartsAt:        startsAt,
		EndsAt:          endsAt,
		Status:          request.Status,
		CoverImageURL:   trimOptionalString(request.CoverImageURL),
		IsArchived:      request.IsArchived,
	}, nil
}

func validateAddPlanningMemberRequest(request AddPlanningMemberRequest) (AddPlanningMemberInput, error) {
	details := make([]response.ErrorDetail, 0)

	trimmedUserID := strings.TrimSpace(request.UserID)
	trimmedEmail := strings.TrimSpace(request.Email)
	var userID uuid.UUID
	if trimmedUserID != "" {
		parsedUserID, err := uuid.Parse(trimmedUserID)
		if err != nil {
			details = append(details, response.ErrorDetail{Field: "user_id", Message: "user_id must be a valid UUID"})
		} else {
			userID = parsedUserID
		}
	}

	if trimmedUserID == "" && trimmedEmail == "" {
		details = append(details, response.ErrorDetail{Field: "user_id", Message: "user_id or email is required"})
	}

	if trimmedEmail != "" && !strings.Contains(trimmedEmail, "@") {
		details = append(details, response.ErrorDetail{Field: "email", Message: "email must be a valid address"})
	}

	role := strings.TrimSpace(request.Role)
	if !isValidMemberRole(role) || role == RoleOwner {
		details = append(details, response.ErrorDetail{Field: "role", Message: fmt.Sprintf("role must be %s or %s", RoleAdmin, RoleMember)})
	}

	if len(details) > 0 {
		return AddPlanningMemberInput{}, ValidationError{Details: details}
	}

	return AddPlanningMemberInput{UserID: userID, Email: strings.ToLower(trimmedEmail), Role: role}, nil
}

func validateUpdatePlanningMemberRoleRequest(request UpdatePlanningMemberRoleRequest) (UpdatePlanningMemberRoleInput, error) {
	role := strings.TrimSpace(request.Role)
	if !isValidMemberRole(role) || role == RoleOwner {
		return UpdatePlanningMemberRoleInput{}, ValidationError{Details: []response.ErrorDetail{{Field: "role", Message: fmt.Sprintf("role must be %s or %s", RoleAdmin, RoleMember)}}}
	}

	return UpdatePlanningMemberRoleInput{Role: role}, nil
}

func validateCreatePlanningRouteStopRequest(request CreatePlanningRouteStopRequest) (CreatePlanningRouteStopInput, error) {
	details := make([]response.ErrorDetail, 0)

	dayID, err := uuid.Parse(strings.TrimSpace(request.DayID))
	if err != nil {
		details = append(details, response.ErrorDetail{Field: "day_id", Message: "day_id must be a valid UUID"})
	}

	title := strings.TrimSpace(request.Title)
	if title == "" {
		details = append(details, response.ErrorDetail{Field: "title", Message: "title is required"})
	}

	if request.DurationMinutes < 0 {
		details = append(details, response.ErrorDetail{Field: "duration_minutes", Message: "duration_minutes must be greater than or equal to 0"})
	}

	if request.CostEstimate < 0 {
		details = append(details, response.ErrorDetail{Field: "cost_estimate", Message: "cost_estimate must be greater than or equal to 0"})
	}

	if len(details) > 0 {
		return CreatePlanningRouteStopInput{}, ValidationError{Details: details}
	}

	return CreatePlanningRouteStopInput{
		DayID:           dayID,
		Title:           title,
		Description:     strings.TrimSpace(request.Description),
		Address:         strings.TrimSpace(request.Address),
		TimeLabel:       strings.TrimSpace(request.TimeLabel),
		DurationMinutes: request.DurationMinutes,
		Notes:           strings.TrimSpace(request.Notes),
		CostEstimate:    request.CostEstimate,
		Currency:        strings.TrimSpace(request.Currency),
		ExternalURL:     strings.TrimSpace(request.ExternalURL),
		ImageURL:        trimOptionalString(request.ImageURL),
		Status:          strings.TrimSpace(request.Status),
		Category:        strings.TrimSpace(request.Category),
		IsOptional:      request.IsOptional,
		Coordinates: PlanningItineraryCoordinates{
			Latitude:  request.Coordinates.Latitude,
			Longitude: request.Coordinates.Longitude,
		},
	}, nil
}

func validateUpdatePlanningRouteStopRequest(request UpdatePlanningRouteStopRequest) (UpdatePlanningRouteStopInput, error) {
	details := make([]response.ErrorDetail, 0)

	var dayID *uuid.UUID
	if request.DayID != nil {
		parsedDayID, err := uuid.Parse(strings.TrimSpace(*request.DayID))
		if err != nil {
			details = append(details, response.ErrorDetail{Field: "day_id", Message: "day_id must be a valid UUID"})
		} else {
			dayID = &parsedDayID
		}
	}

	if request.DurationMinutes != nil && *request.DurationMinutes < 0 {
		details = append(details, response.ErrorDetail{Field: "duration_minutes", Message: "duration_minutes must be greater than or equal to 0"})
	}

	if request.CostEstimate != nil && *request.CostEstimate < 0 {
		details = append(details, response.ErrorDetail{Field: "cost_estimate", Message: "cost_estimate must be greater than or equal to 0"})
	}

	if len(details) > 0 {
		return UpdatePlanningRouteStopInput{}, ValidationError{Details: details}
	}

	input := UpdatePlanningRouteStopInput{
		DayID:           dayID,
		Title:           trimOptionalString(request.Title),
		Description:     trimOptionalString(request.Description),
		Address:         trimOptionalString(request.Address),
		TimeLabel:       trimOptionalString(request.TimeLabel),
		DurationMinutes: request.DurationMinutes,
		Notes:           trimOptionalString(request.Notes),
		CostEstimate:    request.CostEstimate,
		Currency:        trimOptionalString(request.Currency),
		ExternalURL:     trimOptionalString(request.ExternalURL),
		ImageURL:        request.ImageURL,
		Status:          trimOptionalString(request.Status),
		Category:        trimOptionalString(request.Category),
		IsOptional:      request.IsOptional,
	}

	if request.Coordinates != nil {
		input.Coordinates = &PlanningItineraryCoordinates{
			Latitude:  request.Coordinates.Latitude,
			Longitude: request.Coordinates.Longitude,
		}
	}

	return input, nil
}

func validateReorderPlanningRouteStopsRequest(request ReorderPlanningRouteStopsRequest) (ReorderPlanningRouteStopsInput, error) {
	details := make([]response.ErrorDetail, 0)

	dayID, err := uuid.Parse(strings.TrimSpace(request.DayID))
	if err != nil {
		details = append(details, response.ErrorDetail{Field: "day_id", Message: "day_id must be a valid UUID"})
	}

	if len(request.StopIDs) == 0 {
		details = append(details, response.ErrorDetail{Field: "stop_ids", Message: "stop_ids must contain at least one route stop"})
	}

	stopIDs := make([]uuid.UUID, 0, len(request.StopIDs))
	seen := map[uuid.UUID]bool{}
	for index, rawStopID := range request.StopIDs {
		stopID, parseErr := uuid.Parse(strings.TrimSpace(rawStopID))
		if parseErr != nil {
			details = append(details, response.ErrorDetail{Field: fmt.Sprintf("stop_ids[%d]", index), Message: "must be a valid UUID"})
			continue
		}

		if seen[stopID] {
			details = append(details, response.ErrorDetail{Field: fmt.Sprintf("stop_ids[%d]", index), Message: "duplicated route stop id"})
			continue
		}

		seen[stopID] = true
		stopIDs = append(stopIDs, stopID)
	}

	if len(details) > 0 {
		return ReorderPlanningRouteStopsInput{}, ValidationError{Details: details}
	}

	return ReorderPlanningRouteStopsInput{DayID: dayID, StopIDs: stopIDs}, nil
}

func normalizePage(page int, pageSize int) Page {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = defaultPageSize
	}

	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return Page{Number: page, Size: pageSize}
}

func validateDateRange(startsAtRaw *string, endsAtRaw *string) (*time.Time, *time.Time, []response.ErrorDetail) {
	details := make([]response.ErrorDetail, 0)

	startsAt, startErr := parseOptionalTimestamp(startsAtRaw)
	if startErr != nil {
		details = append(details, response.ErrorDetail{Field: "starts_at", Message: startErr.Error()})
	}

	endsAt, endErr := parseOptionalTimestamp(endsAtRaw)
	if endErr != nil {
		details = append(details, response.ErrorDetail{Field: "ends_at", Message: endErr.Error()})
	}

	if startsAt != nil && endsAt != nil && startsAt.After(*endsAt) {
		details = append(details, response.ErrorDetail{Field: "ends_at", Message: "ends_at must be greater than or equal to starts_at"})
	}

	return startsAt, endsAt, details
}

func parseOptionalTimestamp(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	trimmedValue := strings.TrimSpace(*value)
	if trimmedValue == "" {
		return nil, nil
	}

	parsedTime, err := time.Parse(time.RFC3339, trimmedValue)
	if err != nil {
		return nil, fmt.Errorf("must be a valid RFC3339 timestamp")
	}

	return &parsedTime, nil
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func isValidPlanningStatus(status string) bool {
	switch status {
	case StatusDraft, StatusConfirmed, StatusInProgress, StatusFinished:
		return true
	default:
		return false
	}
}

func isValidMemberRole(role string) bool {
	switch role {
	case RoleOwner, RoleAdmin, RoleMember:
		return true
	default:
		return false
	}
}
