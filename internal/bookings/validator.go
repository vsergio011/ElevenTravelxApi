package bookings

import (
	"fmt"
	"github.com/google/uuid"
	"net/url"
	"strings"
	"time"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func validateCreateBookingRequest(request CreateBookingRequest) (CreateBookingInput, error) {
	details := make([]response.ErrorDetail, 0)

	title := strings.TrimSpace(request.Title)
	if title == "" {
		details = append(details, response.ErrorDetail{Field: "title", Message: "title is required"})
	}

	bookingType := strings.TrimSpace(request.Type)
	if !isValidBookingType(bookingType) {
		details = append(details, response.ErrorDetail{Field: "type", Message: fmt.Sprintf("type must be one of %s, %s, %s, %s, %s", TypeHotel, TypeRestaurant, TypeAttraction, TypeCar, TypeActivity)})
	}

	status := StatusProposal
	if request.Status != nil {
		status = strings.TrimSpace(*request.Status)
		if !isValidBookingStatus(status) {
			details = append(details, response.ErrorDetail{Field: "status", Message: fmt.Sprintf("status must be %s or %s", StatusProposal, StatusConfirmed)})
		}
	}

	externalURL, urlErr := validateExternalURL(request.ExternalURL)
	if urlErr != nil {
		details = append(details, response.ErrorDetail{Field: "external_url", Message: urlErr.Error()})
	}

	occursAt, occursErr := parseOptionalTimestamp(request.OccursAt)
	if occursErr != nil {
		details = append(details, response.ErrorDetail{Field: "occurs_at", Message: occursErr.Error()})
	}

	if len(details) > 0 {
		return CreateBookingInput{}, ValidationError{Details: details}
	}
	payments, paymentErr := validatePayments(request.Payments)
	if paymentErr != nil {
		details = append(details, paymentErr...)
	}
	if len(details) > 0 {
		return CreateBookingInput{}, ValidationError{Details: details}
	}

	costCents := int64(0)
	if request.CostCents != nil {
		costCents = *request.CostCents
	}
	distribution := DistributionGroup
	if request.CostDistribution != nil {
		distribution = strings.TrimSpace(*request.CostDistribution)
	}
	if costCents < 0 {
		details = append(details, response.ErrorDetail{Field: "cost_cents", Message: "cost_cents cannot be negative"})
	}
	if !isValidDistribution(distribution) {
		details = append(details, response.ErrorDetail{Field: "cost_distribution", Message: "invalid cost distribution"})
	}
	if distribution == DistributionSelectedParticipants && len(request.ParticipantUserIDs) == 0 {
		details = append(details, response.ErrorDetail{Field: "participant_user_ids", Message: "at least one participant is required"})
	}
	if len(details) > 0 {
		return CreateBookingInput{}, ValidationError{Details: details}
	}

	return CreateBookingInput{
		Title:       title,
		Type:        bookingType,
		Status:      status,
		ExternalURL: externalURL,
		OccursAt:    occursAt,
		Location:    trimOptionalString(request.Location),
		Notes:       trimOptionalString(request.Notes),
		CostCents:   costCents, CostDistribution: distribution, ParticipantUserIDs: request.ParticipantUserIDs, RouteStopID: request.RouteStopID, Payments: payments,
	}, nil
}

func validateUpdateBookingRequest(request UpdateBookingRequest) (UpdateBookingInput, error) {
	details := make([]response.ErrorDetail, 0)

	title := trimOptionalString(request.Title)
	if title != nil && *title == "" {
		details = append(details, response.ErrorDetail{Field: "title", Message: "title cannot be empty"})
	}

	var bookingType *string
	if request.Type != nil {
		trimmedType := strings.TrimSpace(*request.Type)
		if !isValidBookingType(trimmedType) {
			details = append(details, response.ErrorDetail{Field: "type", Message: fmt.Sprintf("type must be one of %s, %s, %s, %s, %s", TypeHotel, TypeRestaurant, TypeAttraction, TypeCar, TypeActivity)})
		} else {
			bookingType = &trimmedType
		}
	}

	var status *string
	if request.Status != nil {
		trimmedStatus := strings.TrimSpace(*request.Status)
		if !isValidBookingStatus(trimmedStatus) {
			details = append(details, response.ErrorDetail{Field: "status", Message: "status must be proposal or confirmed"})
		} else {
			status = &trimmedStatus
		}
	}

	externalURL, urlErr := validateExternalURL(request.ExternalURL)
	if urlErr != nil {
		details = append(details, response.ErrorDetail{Field: "external_url", Message: urlErr.Error()})
	}

	occursAt, occursErr := parseOptionalTimestamp(request.OccursAt)
	if occursErr != nil {
		details = append(details, response.ErrorDetail{Field: "occurs_at", Message: occursErr.Error()})
	}

	if len(details) > 0 {
		return UpdateBookingInput{}, ValidationError{Details: details}
	}
	var payments *[]Payment
	if request.Payments != nil {
		parsed, paymentErr := validatePayments(*request.Payments)
		if paymentErr != nil {
			details = append(details, paymentErr...)
		} else {
			payments = &parsed
		}
	}
	if len(details) > 0 {
		return UpdateBookingInput{}, ValidationError{Details: details}
	}

	if request.CostCents != nil && *request.CostCents < 0 {
		details = append(details, response.ErrorDetail{Field: "cost_cents", Message: "cost_cents cannot be negative"})
	}
	if request.CostDistribution != nil && !isValidDistribution(strings.TrimSpace(*request.CostDistribution)) {
		details = append(details, response.ErrorDetail{Field: "cost_distribution", Message: "invalid cost distribution"})
	}
	if request.CostDistribution != nil && strings.TrimSpace(*request.CostDistribution) == DistributionSelectedParticipants && (request.ParticipantUserIDs == nil || len(*request.ParticipantUserIDs) == 0) {
		details = append(details, response.ErrorDetail{Field: "participant_user_ids", Message: "at least one participant is required"})
	}
	if len(details) > 0 {
		return UpdateBookingInput{}, ValidationError{Details: details}
	}

	return UpdateBookingInput{
		Title:       title,
		Type:        bookingType,
		Status:      status,
		ExternalURL: externalURL,
		OccursAt:    occursAt,
		Location:    trimOptionalString(request.Location),
		Notes:       trimOptionalString(request.Notes),
		CostCents:   request.CostCents, CostDistribution: request.CostDistribution, ParticipantUserIDs: request.ParticipantUserIDs, RouteStopID: request.RouteStopID, Payments: payments,
	}, nil
}

func validatePayments(requests []PaymentRequest) ([]Payment, []response.ErrorDetail) {
	result := make([]Payment, 0, len(requests))
	details := make([]response.ErrorDetail, 0)
	for i, item := range requests {
		if item.UserID == uuid.Nil {
			details = append(details, response.ErrorDetail{Field: fmt.Sprintf("payments[%d].user_id", i), Message: "user_id is required"})
		}
		if item.AmountCents <= 0 {
			details = append(details, response.ErrorDetail{Field: fmt.Sprintf("payments[%d].amount_cents", i), Message: "amount_cents must be greater than zero"})
			continue
		}
		paidAt := time.Now().UTC()
		if item.PaidAt != nil && strings.TrimSpace(*item.PaidAt) != "" {
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*item.PaidAt))
			if err != nil {
				details = append(details, response.ErrorDetail{Field: fmt.Sprintf("payments[%d].paid_at", i), Message: "must be a valid RFC3339 timestamp"})
				continue
			}
			paidAt = parsed
		}
		result = append(result, Payment{UserID: item.UserID, AmountCents: item.AmountCents, PaidAt: paidAt})
	}
	return result, details
}

func validateExternalURL(value *string) (*string, error) {
	trimmed := trimOptionalString(value)
	if trimmed == nil || *trimmed == "" {
		return trimmed, nil
	}

	parsedURL, err := url.ParseRequestURI(*trimmed)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, fmt.Errorf("external_url must be a valid http(s) URL")
	}

	return trimmed, nil
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

func isValidBookingType(bookingType string) bool {
	switch bookingType {
	case TypeHotel, TypeRestaurant, TypeAttraction, TypeCar, TypeActivity:
		return true
	default:
		return false
	}
}

func isValidBookingStatus(status string) bool {
	switch status {
	case StatusProposal, StatusConfirmed:
		return true
	default:
		return false
	}
}

func isValidDistribution(value string) bool {
	return value == DistributionIndividual || value == DistributionGroup || value == DistributionSelectedParticipants
}
