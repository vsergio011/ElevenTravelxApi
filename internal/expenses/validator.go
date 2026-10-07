package expenses

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

const maxAmountCents = 100_000_000_00

func isValidDistribution(value string) bool {
	return value == DistributionGroup || value == DistributionSelectedParticipants
}

func parseUserIDs(raw []string) ([]uuid.UUID, []response.ErrorDetail) {
	ids := make([]uuid.UUID, 0, len(raw))
	seen := map[uuid.UUID]struct{}{}
	for _, value := range raw {
		id, err := uuid.Parse(strings.TrimSpace(value))
		if err != nil {
			return nil, []response.ErrorDetail{{Field: "participant_user_ids", Message: "participant_user_ids must contain valid UUIDs"}}
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseOptionalTime(value *string) (*time.Time, *response.ErrorDetail) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil, &response.ErrorDetail{Field: "spent_at", Message: "spent_at must be an RFC3339 datetime"}
	}
	return &parsed, nil
}

func cleanOptional(value *string, max int, field string, details *[]response.ErrorDetail) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if len(trimmed) > max {
		*details = append(*details, response.ErrorDetail{Field: field, Message: field + " is too long"})
	}
	return &trimmed
}

func validateTitle(title string, details *[]response.ErrorDetail) string {
	trimmed := strings.TrimSpace(title)
	if len(trimmed) < 2 || len(trimmed) > 140 {
		*details = append(*details, response.ErrorDetail{Field: "title", Message: "title must be between 2 and 140 characters"})
	}
	return trimmed
}

func validateAmount(amount int64, details *[]response.ErrorDetail) {
	if amount <= 0 || amount > maxAmountCents {
		*details = append(*details, response.ErrorDetail{Field: "amount_cents", Message: "amount_cents must be a positive amount"})
	}
}

func parsePayer(raw string, details *[]response.ErrorDetail) uuid.UUID {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		*details = append(*details, response.ErrorDetail{Field: "paid_by_user_id", Message: "paid_by_user_id must be a valid UUID"})
	}
	return id
}

func validateCreateRequest(request CreateExpenseRequest) (CreateExpenseInput, error) {
	details := []response.ErrorDetail{}
	input := CreateExpenseInput{Distribution: strings.TrimSpace(request.Distribution)}
	if input.Distribution == "" {
		input.Distribution = DistributionGroup
	}
	input.Title = validateTitle(request.Title, &details)
	validateAmount(request.AmountCents, &details)
	input.AmountCents = request.AmountCents
	input.PaidByUserID = parsePayer(request.PaidByUserID, &details)
	if !isValidDistribution(input.Distribution) {
		details = append(details, response.ErrorDetail{Field: "distribution", Message: "distribution must be group or selected_participants"})
	}
	participants, participantDetails := parseUserIDs(request.ParticipantUserIDs)
	details = append(details, participantDetails...)
	input.ParticipantUserIDs = participants
	if input.Distribution == DistributionSelectedParticipants && len(participants) == 0 && len(participantDetails) == 0 {
		details = append(details, response.ErrorDetail{Field: "participant_user_ids", Message: "select at least one participant"})
	}
	if input.Distribution == DistributionGroup {
		input.ParticipantUserIDs = []uuid.UUID{}
	}
	input.Category = cleanOptional(request.Category, 60, "category", &details)
	input.Notes = cleanOptional(request.Notes, 1000, "notes", &details)
	spentAt, detail := parseOptionalTime(request.SpentAt)
	if detail != nil {
		details = append(details, *detail)
	}
	input.SpentAt = spentAt
	if len(details) > 0 {
		return CreateExpenseInput{}, ValidationError{Details: details}
	}
	return input, nil
}

func validateUpdateRequest(request UpdateExpenseRequest) (UpdateExpenseInput, error) {
	details := []response.ErrorDetail{}
	input := UpdateExpenseInput{}
	if request.Title != nil {
		title := validateTitle(*request.Title, &details)
		input.Title = &title
	}
	if request.AmountCents != nil {
		validateAmount(*request.AmountCents, &details)
		input.AmountCents = request.AmountCents
	}
	if request.PaidByUserID != nil {
		payer := parsePayer(*request.PaidByUserID, &details)
		input.PaidByUserID = &payer
	}
	if request.Distribution != nil {
		distribution := strings.TrimSpace(*request.Distribution)
		if !isValidDistribution(distribution) {
			details = append(details, response.ErrorDetail{Field: "distribution", Message: "distribution must be group or selected_participants"})
		}
		input.Distribution = &distribution
	}
	if request.ParticipantUserIDs != nil {
		participants, participantDetails := parseUserIDs(*request.ParticipantUserIDs)
		details = append(details, participantDetails...)
		input.ParticipantUserIDs = &participants
	}
	input.Category = cleanOptional(request.Category, 60, "category", &details)
	input.Notes = cleanOptional(request.Notes, 1000, "notes", &details)
	spentAt, detail := parseOptionalTime(request.SpentAt)
	if detail != nil {
		details = append(details, *detail)
	}
	input.SpentAt = spentAt
	if len(details) > 0 {
		return UpdateExpenseInput{}, ValidationError{Details: details}
	}
	return input, nil
}
