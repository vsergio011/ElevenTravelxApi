package urlpreview

import (
	"context"
	"errors"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/google/uuid"
)

var (
	ErrForbidden  = errors.New("forbidden")
	ErrInvalidURL = errors.New("invalid_url")
)

type PlanningMembership interface {
	GetMember(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error)
}

type PreviewRequest struct {
	URL string `json:"url"`
}

type PreviewResponse struct {
	URL                    string  `json:"url"`
	SourceDomain           string  `json:"source_domain"`
	Title                  *string `json:"title,omitempty"`
	Description            *string `json:"description,omitempty"`
	Location               *string `json:"location,omitempty"`
	StartsAt               *string `json:"starts_at,omitempty"`
	EndsAt                 *string `json:"ends_at,omitempty"`
	SuggestedBookingType   *string `json:"suggested_booking_type,omitempty"`
	SuggestedTransportType *string `json:"suggested_transport_type,omitempty"`
	Operator               *string `json:"operator,omitempty"`
	PriceCents             *int64  `json:"price_cents,omitempty"`
	Currency               *string `json:"currency,omitempty"`
}
