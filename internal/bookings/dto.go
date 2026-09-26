package bookings

import (
	"time"

	"github.com/google/uuid"
)

type CreateBookingRequest struct {
	Title              string           `json:"title"`
	Type               string           `json:"type"`
	Status             *string          `json:"status"`
	ExternalURL        *string          `json:"external_url"`
	OccursAt           *string          `json:"occurs_at"`
	Location           *string          `json:"location"`
	Notes              *string          `json:"notes"`
	CostCents          *int64           `json:"cost_cents"`
	CostDistribution   *string          `json:"cost_distribution"`
	ParticipantUserIDs []uuid.UUID      `json:"participant_user_ids"`
	RouteStopID        *uuid.UUID       `json:"route_stop_id"`
	Payments           []PaymentRequest `json:"payments"`
}

type UpdateBookingRequest struct {
	Title              *string           `json:"title"`
	Type               *string           `json:"type"`
	Status             *string           `json:"status"`
	ExternalURL        *string           `json:"external_url"`
	OccursAt           *string           `json:"occurs_at"`
	Location           *string           `json:"location"`
	Notes              *string           `json:"notes"`
	CostCents          *int64            `json:"cost_cents"`
	CostDistribution   *string           `json:"cost_distribution"`
	ParticipantUserIDs *[]uuid.UUID      `json:"participant_user_ids"`
	RouteStopID        *uuid.UUID        `json:"route_stop_id"`
	Payments           *[]PaymentRequest `json:"payments"`
}

type PaymentRequest struct {
	UserID      uuid.UUID `json:"user_id"`
	AmountCents int64     `json:"amount_cents"`
	PaidAt      *string   `json:"paid_at"`
}

type BookingResponse struct {
	ID                 uuid.UUID         `json:"id"`
	PlanningID         uuid.UUID         `json:"planning_id"`
	CreatedByUserID    uuid.UUID         `json:"created_by_user_id"`
	Title              string            `json:"title"`
	Type               string            `json:"type"`
	Status             string            `json:"status"`
	ExternalURL        *string           `json:"external_url"`
	OccursAt           *time.Time        `json:"occurs_at"`
	Location           *string           `json:"location"`
	Notes              *string           `json:"notes"`
	CostCents          int64             `json:"cost_cents"`
	CostDistribution   string            `json:"cost_distribution"`
	ParticipantUserIDs []uuid.UUID       `json:"participant_user_ids"`
	RouteStopID        *uuid.UUID        `json:"route_stop_id"`
	Payments           []PaymentResponse `json:"payments"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

func NewBookingResponse(booking Booking) BookingResponse {
	return BookingResponse{
		ID:                 booking.ID,
		PlanningID:         booking.PlanningID,
		CreatedByUserID:    booking.CreatedByUserID,
		Title:              booking.Title,
		Type:               booking.Type,
		Status:             booking.Status,
		ExternalURL:        booking.ExternalURL,
		OccursAt:           booking.OccursAt,
		Location:           booking.Location,
		Notes:              booking.Notes,
		CostCents:          booking.CostCents,
		CostDistribution:   booking.CostDistribution,
		ParticipantUserIDs: booking.ParticipantUserIDs,
		RouteStopID:        booking.RouteStopID,
		Payments:           newPaymentResponses(booking.Payments),
		CreatedAt:          booking.CreatedAt,
		UpdatedAt:          booking.UpdatedAt,
	}
}

type PaymentResponse struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	AmountCents int64     `json:"amount_cents"`
	PaidAt      time.Time `json:"paid_at"`
}

func newPaymentResponses(payments []Payment) []PaymentResponse {
	result := make([]PaymentResponse, 0, len(payments))
	for _, p := range payments {
		result = append(result, PaymentResponse{ID: p.ID, UserID: p.UserID, AmountCents: p.AmountCents, PaidAt: p.PaidAt})
	}
	return result
}

type BudgetLineResponse struct {
	UserID uuid.UUID `json:"user_id"`
	Cents  int64     `json:"cents"`
}

type BudgetResponse struct {
	Data []BudgetLineResponse `json:"data"`
}
