package flights

import (
	"context"
	"time"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/google/uuid"
)

const (
	StatusDraft                      = "draft"
	StatusConfirmed                  = "confirmed"
	DirectionOutbound                = "outbound"
	DirectionReturn                  = "return"
	DistributionIndividual           = "individual"
	DistributionGroup                = "group"
	DistributionSelectedParticipants = "selected_participants"
)

type Segment struct {
	ID                     uuid.UUID
	Direction              string
	Position               int
	OriginLabel            string
	OriginCity             *string
	OriginAirportName      *string
	OriginAirportCode      *string
	OriginMapboxID         *string
	DestinationLabel       string
	DestinationCity        *string
	DestinationAirportName *string
	DestinationAirportCode *string
	DestinationMapboxID    *string
	DepartureAt            time.Time
	ArrivalAt              time.Time
	DepartureTerminal      *string
	ArrivalTerminal        *string
	Airline                *string
	FlightNumber           *string
	OriginTimezone         *string
	DestinationTimezone    *string
}

type Flight struct {
	ID                 uuid.UUID
	PlanningID         uuid.UUID
	CreatedByUserID    uuid.UUID
	Status             string
	Airline            *string
	FlightNumber       *string
	ReservationCode    *string
	Notes              *string
	OfferURL           *string
	CabinClass         *string
	Baggage            *string
	CostCents          int64
	Currency           string
	CostDistribution   string
	ParticipantUserIDs []uuid.UUID
	Segments           []Segment
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateInput struct {
	Status             string
	Airline            *string
	FlightNumber       *string
	ReservationCode    *string
	Notes              *string
	OfferURL           *string
	CabinClass         *string
	Baggage            *string
	CostCents          int64
	Currency           string
	CostDistribution   string
	ParticipantUserIDs []uuid.UUID
	Segments           []Segment
}

type UpdateInput struct {
	Status             *string
	Airline            *string
	FlightNumber       *string
	ReservationCode    *string
	Notes              *string
	OfferURL           *string
	CabinClass         *string
	Baggage            *string
	CostCents          *int64
	Currency           *string
	CostDistribution   *string
	ParticipantUserIDs *[]uuid.UUID
	Segments           *[]Segment
}

type Filters struct{ Status *string }
type BudgetLine struct {
	UserID uuid.UUID
	Cents  int64
}

// PlanningMembership is the existing planning permission system.
type PlanningMembership interface {
	GetMember(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error)
}
