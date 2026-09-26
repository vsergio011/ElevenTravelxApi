package plannings

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

const (
	StatusDraft      = "draft"
	StatusConfirmed  = "confirmed"
	StatusInProgress = "in_progress"
	StatusFinished   = "finished"
)

type Planning struct {
	ID              uuid.UUID
	GroupID         *uuid.UUID
	OwnerUserID     uuid.UUID
	Name            string
	Description     *string
	DestinationName *string
	StartsAt        *time.Time
	EndsAt          *time.Time
	Status          string
	CoverImageURL   *string
	IsArchived      bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PlanningMember struct {
	ID              uuid.UUID
	PlanningID      uuid.UUID
	UserID          uuid.UUID
	Username        *string
	FullName        *string
	AvatarURL       *string
	Role            string
	InvitedByUserID *uuid.UUID
	JoinedAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PlanningActivity struct {
	ID          uuid.UUID
	PlanningID  uuid.UUID
	ActorUserID *uuid.UUID
	EventType   string
	EntityType  string
	EntityID    *uuid.UUID
	Metadata    map[string]any
	CreatedAt   time.Time
}

type PlanningUpcomingActivity struct {
	ID        uuid.UUID
	Title     string
	StartsAt  time.Time
	KindLabel *string
	ImageURL  *string
}

type PlanningNextStop struct {
	ID          uuid.UUID
	Title       string
	TimeLabel   *string
	Location    *string
	Description *string
	ImageURL    *string
}

type PlanningDashboardActivity struct {
	PlanningID         uuid.UUID
	UpcomingActivities []PlanningUpcomingActivity
	NextStop           *PlanningNextStop
}

type PlanningSummaryBudget struct {
	Target    int
	Confirmed int
	Pending   int
}

type PlanningSummaryShortcuts struct {
	FlightsCount  int
	BookingsCount int
	ExpensesCount int
}

type PlanningSummary struct {
	PlanningID uuid.UUID
	Budget     *PlanningSummaryBudget
	Shortcuts  PlanningSummaryShortcuts
}

type PlanningItineraryCoordinates struct {
	Latitude  float64
	Longitude float64
}

type PlanningItineraryStop struct {
	ID              uuid.UUID
	Title           string
	Description     string
	Address         string
	TimeLabel       string
	DurationMinutes int
	Notes           string
	CostEstimate    float64
	Currency        string
	ExternalURL     string
	ImageURL        *string
	Status          string
	Category        string
	IsOptional      bool
	Coordinates     PlanningItineraryCoordinates
	Bookings        []PlanningRouteBooking
}

type PlanningRouteBooking struct {
	ID       uuid.UUID
	Title    string
	Type     string
	Status   string
	OccursAt *time.Time
}

type PlanningItineraryDay struct {
	ID        uuid.UUID
	Label     string
	DateLabel string
	Stops     []PlanningItineraryStop
}

type PlanningItinerary struct {
	PlanningID   uuid.UUID
	PlanningName string
	Days         []PlanningItineraryDay
}

type CreatePlanningRouteDayInput struct {
	Label     string
	DateLabel string
}

type CreatePlanningRouteStopInput struct {
	DayID           uuid.UUID
	Title           string
	Description     string
	Address         string
	TimeLabel       string
	DurationMinutes int
	Notes           string
	CostEstimate    float64
	Currency        string
	ExternalURL     string
	ImageURL        *string
	Status          string
	Category        string
	IsOptional      bool
	Coordinates     PlanningItineraryCoordinates
}

type UpdatePlanningRouteStopInput struct {
	DayID           *uuid.UUID
	Title           *string
	Description     *string
	Address         *string
	TimeLabel       *string
	DurationMinutes *int
	Notes           *string
	CostEstimate    *float64
	Currency        *string
	ExternalURL     *string
	ImageURL        *string
	Status          *string
	Category        *string
	IsOptional      *bool
	Coordinates     *PlanningItineraryCoordinates
}

type ReorderPlanningRouteStopsInput struct {
	DayID   uuid.UUID
	StopIDs []uuid.UUID
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

type PlanningFilters struct {
	Page       Page
	GroupID    *uuid.UUID
	Status     *string
	Archived   *bool
	OnlyMember uuid.UUID
}

type CreatePlanningInput struct {
	GroupID         *uuid.UUID
	Name            string
	Description     *string
	DestinationName *string
	StartsAt        *time.Time
	EndsAt          *time.Time
}

type UpdatePlanningInput struct {
	Name            *string
	Description     *string
	DestinationName *string
	StartsAt        *time.Time
	EndsAt          *time.Time
	Status          *string
	CoverImageURL   *string
	IsArchived      *bool
}

type AddPlanningMemberInput struct {
	UserID uuid.UUID
	Email  string
	Role   string
}

type UpdatePlanningMemberRoleInput struct {
	Role string
}
