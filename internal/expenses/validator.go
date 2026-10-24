package expenses

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

const maxAmountCents = 100_000_000_00

func isValidDistribution(value string) bool {
	return value == DistributionGroup || value == DistributionSelectedParticipants ||
		value == DistributionIndividual || value == DistributionCustom
}

func isValidCurrency(value string) bool {
	return value == "EUR" || value == "USD"
}

func parsePayments(raw []PaymentRequest) ([]Payment, []response.ErrorDetail) {
	payments := make([]Payment, 0, len(raw))
	for _, item := range raw {
		id, err := uuid.Parse(strings.TrimSpace(item.UserID))
		if err != nil {
			return nil, []response.ErrorDetail{{Field: "payments", Message: "payments must contain valid user_id UUIDs"}}
		}
		if item.AmountCents <= 0 || item.AmountCents > maxAmountCents {
			return nil, []response.ErrorDetail{{Field: "payments", Message: "payment amount_cents must be a positive amount"}}
		}
		payment := Payment{UserID: id, AmountCents: item.AmountCents}
		paidAt, detail := parseOptionalTime(item.PaidAt)
		if detail != nil {
			return nil, []response.ErrorDetail{{Field: "payments", Message: "paid_at must be an RFC3339 datetime"}}
		}
		if paidAt != nil {
			payment.PaidAt = *paidAt
		}
		payments = append(payments, payment)
	}
	return payments, nil
}

func parseShares(raw []ShareRequest) ([]Share, []response.ErrorDetail) {
	shares := make([]Share, 0, len(raw))
	for _, item := range raw {
		id, err := uuid.Parse(strings.TrimSpace(item.UserID))
		if err != nil || item.AmountCents < 0 {
			return nil, []response.ErrorDetail{{Field: "shares", Message: "shares must contain valid user_id and non-negative amount_cents"}}
		}
		shares = append(shares, Share{UserID: id, Cents: item.AmountCents})
	}
	return shares, nil
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
	input := CreateExpenseInput{Distribution: strings.TrimSpace(request.Distribution), Currency: strings.ToUpper(strings.TrimSpace(request.Currency))}
	if input.Distribution == "" {
		input.Distribution = DistributionGroup
	}
	if input.Currency == "" {
		input.Currency = "EUR"
	}
	if !isValidCurrency(input.Currency) {
		details = append(details, response.ErrorDetail{Field: "currency", Message: "currency must be EUR or USD"})
	}
	input.Title = validateTitle(request.Title, &details)
	validateAmount(request.AmountCents, &details)
	input.AmountCents = request.AmountCents
	if request.PlanningItemID != nil && strings.TrimSpace(*request.PlanningItemID) != "" {
		itemID, err := uuid.Parse(strings.TrimSpace(*request.PlanningItemID))
		if err != nil {
			details = append(details, response.ErrorDetail{Field: "planning_item_id", Message: "planning_item_id must be a valid UUID"})
		} else {
			input.PlanningItemID = &itemID
		}
	}
	payments, paymentDetails := parsePayments(request.Payments)
	details = append(details, paymentDetails...)
	input.Payments = payments
	if len(request.Payments) == 0 && strings.TrimSpace(request.PaidByUserID) != "" {
		payer := parsePayer(request.PaidByUserID, &details)
		input.Payments = []Payment{{UserID: payer, AmountCents: request.AmountCents}}
	}
	if !isValidDistribution(input.Distribution) {
		details = append(details, response.ErrorDetail{Field: "distribution", Message: "distribution must be group, selected_participants, individual or custom"})
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
	shares, shareDetails := parseShares(request.Shares)
	details = append(details, shareDetails...)
	input.CustomShares = shares
	if input.Distribution == DistributionCustom && len(shares) == 0 && len(shareDetails) == 0 {
		details = append(details, response.ErrorDetail{Field: "shares", Message: "custom distribution requires shares"})
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
	if request.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*request.Currency))
		if !isValidCurrency(currency) {
			details = append(details, response.ErrorDetail{Field: "currency", Message: "currency must be EUR or USD"})
		}
		input.Currency = &currency
	}
	if request.Payments != nil {
		payments, paymentDetails := parsePayments(*request.Payments)
		details = append(details, paymentDetails...)
		input.Payments = &payments
	} else if request.PaidByUserID != nil && request.AmountCents != nil {
		payer := parsePayer(*request.PaidByUserID, &details)
		payments := []Payment{{UserID: payer, AmountCents: *request.AmountCents}}
		input.Payments = &payments
	}
	if request.Shares != nil {
		shares, shareDetails := parseShares(*request.Shares)
		details = append(details, shareDetails...)
		input.CustomShares = &shares
	}
	if request.Distribution != nil {
		distribution := strings.TrimSpace(*request.Distribution)
		if !isValidDistribution(distribution) {
			details = append(details, response.ErrorDetail{Field: "distribution", Message: "distribution must be group, selected_participants, individual or custom"})
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
