package bookings

import (
	"time"

	"github.com/google/uuid"
)

const (
	TypeHotel      = "hotel"
	TypeRestaurant = "restaurant"
	TypeAttraction = "attraction"
	TypeCar        = "car"
	TypeActivity   = "activity"
)

const (
	StatusProposal  = "proposal"
	StatusConfirmed = "confirmed"
)

type Booking struct {
	ID                 uuid.UUID
	PlanningID         uuid.UUID
	CreatedByUserID    uuid.UUID
	Title              string
	Type               string
	Status             string
	ExternalURL        *string
	OccursAt           *time.Time
	Location           *string
	Notes              *string
	CostCents          int64
	CostDistribution   string
	ParticipantUserIDs []uuid.UUID
	RouteStopID        *uuid.UUID
	Payments           []Payment
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Payment struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	AmountCents int64
	PaidAt      time.Time
}

type Page struct {
	Number int
	Size   int
}

type PageResult[T any] struct {
	Items      []T
	Page       int
	PageSize   int
	Total      int
	TotalPages int
}

type BudgetLine struct {
	UserID uuid.UUID
	Cents  int64
}

type BookingFilters struct {
	Page   Page
	Status *string
	Type   *string
}

type CreateBookingInput struct {
	Title              string
	Type               string
	Status             string
	ExternalURL        *string
	OccursAt           *time.Time
	Location           *string
	Notes              *string
	CostCents          int64
	CostDistribution   string
	ParticipantUserIDs []uuid.UUID
	RouteStopID        *uuid.UUID
	Payments           []Payment
}

type UpdateBookingInput struct {
	Title              *string
	Type               *string
	Status             *string
	ExternalURL        *string
	OccursAt           *time.Time
	Location           *string
	Notes              *string
	CostCents          *int64
	CostDistribution   *string
	ParticipantUserIDs *[]uuid.UUID
	RouteStopID        *uuid.UUID
	Payments           *[]Payment
}

const (
	DistributionIndividual           = "individual"
	DistributionGroup                = "group"
	DistributionSelectedParticipants = "selected_participants"
)
