package plannings

import (
	"time"

	"github.com/google/uuid"
)

type CreatePlanningRequest struct {
	GroupID         string  `json:"group_id"`
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	DestinationName *string `json:"destination_name"`
	StartsAt        *string `json:"starts_at"`
	EndsAt          *string `json:"ends_at"`
}

type UpdatePlanningRequest struct {
	Name            *string `json:"name"`
	Description     *string `json:"description"`
	DestinationName *string `json:"destination_name"`
	StartsAt        *string `json:"starts_at"`
	EndsAt          *string `json:"ends_at"`
	Status          *string `json:"status"`
	CoverImageURL   *string `json:"cover_image_url"`
	IsArchived      *bool   `json:"is_archived"`
}

type CreatePlanningRouteDayRequest struct {
	Label     string `json:"label"`
	DateLabel string `json:"date_label"`
}

type AddPlanningMemberRequest struct {
	UserID string `json:"user_id,omitempty"`
	Email  string `json:"email,omitempty"`
	Role   string `json:"role"`
}

type UpdatePlanningMemberRoleRequest struct {
	Role string `json:"role"`
}

type PlanningItineraryCoordinatesRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type CreatePlanningRouteStopRequest struct {
	DayID           string                              `json:"day_id"`
	Title           string                              `json:"title"`
	Description     string                              `json:"description"`
	Address         string                              `json:"address"`
	TimeLabel       string                              `json:"time_label"`
	DurationMinutes int                                 `json:"duration_minutes"`
	Notes           string                              `json:"notes"`
	CostEstimate    float64                             `json:"cost_estimate"`
	Currency        string                              `json:"currency"`
	ExternalURL     string                              `json:"external_url"`
	ImageURL        *string                             `json:"image_url"`
	Status          string                              `json:"status"`
	Category        string                              `json:"category"`
	IsOptional      bool                                `json:"is_optional"`
	Coordinates     PlanningItineraryCoordinatesRequest `json:"coordinates"`
}

type UpdatePlanningRouteStopRequest struct {
	DayID           *string                              `json:"day_id"`
	Title           *string                              `json:"title"`
	Description     *string                              `json:"description"`
	Address         *string                              `json:"address"`
	TimeLabel       *string                              `json:"time_label"`
	DurationMinutes *int                                 `json:"duration_minutes"`
	Notes           *string                              `json:"notes"`
	CostEstimate    *float64                             `json:"cost_estimate"`
	Currency        *string                              `json:"currency"`
	ExternalURL     *string                              `json:"external_url"`
	ImageURL        *string                              `json:"image_url"`
	Status          *string                              `json:"status"`
	Category        *string                              `json:"category"`
	IsOptional      *bool                                `json:"is_optional"`
	Coordinates     *PlanningItineraryCoordinatesRequest `json:"coordinates"`
}

type ReorderPlanningRouteStopsRequest struct {
	DayID   string   `json:"day_id"`
	StopIDs []string `json:"stop_ids"`
}

type PlanningResponse struct {
	ID              uuid.UUID  `json:"id"`
	GroupID         *uuid.UUID `json:"group_id"`
	OwnerUserID     uuid.UUID  `json:"owner_user_id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description"`
	DestinationName *string    `json:"destination_name"`
	StartsAt        *time.Time `json:"starts_at"`
	EndsAt          *time.Time `json:"ends_at"`
	Status          string     `json:"status"`
	CoverImageURL   *string    `json:"cover_image_url"`
	IsArchived      bool       `json:"is_archived"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type PlanningMemberResponse struct {
	ID              uuid.UUID  `json:"id"`
	PlanningID      uuid.UUID  `json:"planning_id"`
	UserID          uuid.UUID  `json:"user_id"`
	Username        *string    `json:"username"`
	FullName        *string    `json:"full_name"`
	AvatarURL       *string    `json:"avatar_url"`
	Role            string     `json:"role"`
	InvitedByUserID *uuid.UUID `json:"invited_by_user_id"`
	JoinedAt        *time.Time `json:"joined_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type PlanningActivityResponse struct {
	ID          uuid.UUID      `json:"id"`
	PlanningID  uuid.UUID      `json:"planning_id"`
	ActorUserID *uuid.UUID     `json:"actor_user_id"`
	EventType   string         `json:"event_type"`
	EntityType  string         `json:"entity_type"`
	EntityID    *uuid.UUID     `json:"entity_id"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type PlanningUpcomingActivityResponse struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	StartsAt  time.Time `json:"starts_at"`
	KindLabel *string   `json:"kind_label"`
	ImageURL  *string   `json:"image_url"`
}

type PlanningNextStopResponse struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	TimeLabel   *string   `json:"time_label"`
	Location    *string   `json:"location"`
	Description *string   `json:"description"`
	ImageURL    *string   `json:"image_url"`
}

type PlanningDashboardActivityResponse struct {
	PlanningID         uuid.UUID                          `json:"planning_id"`
	UpcomingActivities []PlanningUpcomingActivityResponse `json:"upcoming_activities"`
	NextStop           *PlanningNextStopResponse          `json:"next_stop"`
}

type PlanningSummaryBudgetResponse struct {
	Target    int `json:"target"`
	Confirmed int `json:"confirmed"`
	Pending   int `json:"pending"`
}

type PlanningSummaryShortcutsResponse struct {
	FlightsCount  int `json:"flights_count"`
	BookingsCount int `json:"bookings_count"`
	ExpensesCount int `json:"expenses_count"`
}

type PlanningSummaryResponse struct {
	PlanningID uuid.UUID                        `json:"planning_id"`
	Budget     *PlanningSummaryBudgetResponse   `json:"budget"`
	Shortcuts  PlanningSummaryShortcutsResponse `json:"shortcuts"`
}

type PlanningItineraryCoordinatesResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type PlanningItineraryStopResponse struct {
	ID              uuid.UUID                            `json:"id"`
	Title           string                               `json:"title"`
	Description     string                               `json:"description"`
	Address         string                               `json:"address"`
	TimeLabel       string                               `json:"time_label"`
	DurationMinutes int                                  `json:"duration_minutes"`
	Notes           string                               `json:"notes"`
	CostEstimate    float64                              `json:"cost_estimate"`
	Currency        string                               `json:"currency"`
	ExternalURL     string                               `json:"external_url"`
	ImageURL        *string                              `json:"image_url,omitempty"`
	Status          string                               `json:"status"`
	Category        string                               `json:"category"`
	IsOptional      bool                                 `json:"is_optional"`
	Coordinates     PlanningItineraryCoordinatesResponse `json:"coordinates"`
	Bookings        []PlanningRouteBookingResponse       `json:"bookings"`
}

type PlanningRouteBookingResponse struct {
	ID       uuid.UUID  `json:"id"`
	Title    string     `json:"title"`
	Type     string     `json:"type"`
	Status   string     `json:"status"`
	OccursAt *time.Time `json:"occurs_at"`
}

type PlanningItineraryDayResponse struct {
	ID        uuid.UUID                       `json:"id"`
	Label     string                          `json:"label"`
	DateLabel string                          `json:"date_label"`
	Stops     []PlanningItineraryStopResponse `json:"stops"`
}

type PlanningItineraryResponse struct {
	PlanningID   uuid.UUID                      `json:"planning_id"`
	PlanningName string                         `json:"planning_name"`
	Days         []PlanningItineraryDayResponse `json:"days"`
}

func NewPlanningResponse(planning Planning) PlanningResponse {
	return PlanningResponse{
		ID:              planning.ID,
		GroupID:         planning.GroupID,
		OwnerUserID:     planning.OwnerUserID,
		Name:            planning.Name,
		Description:     planning.Description,
		DestinationName: planning.DestinationName,
		StartsAt:        planning.StartsAt,
		EndsAt:          planning.EndsAt,
		Status:          planning.Status,
		CoverImageURL:   planning.CoverImageURL,
		IsArchived:      planning.IsArchived,
		CreatedAt:       planning.CreatedAt,
		UpdatedAt:       planning.UpdatedAt,
	}
}

func NewPlanningMemberResponse(member PlanningMember) PlanningMemberResponse {
	return PlanningMemberResponse{
		ID:              member.ID,
		PlanningID:      member.PlanningID,
		UserID:          member.UserID,
		Username:        member.Username,
		FullName:        member.FullName,
		AvatarURL:       member.AvatarURL,
		Role:            member.Role,
		InvitedByUserID: member.InvitedByUserID,
		JoinedAt:        member.JoinedAt,
		CreatedAt:       member.CreatedAt,
		UpdatedAt:       member.UpdatedAt,
	}
}

func NewPlanningActivityResponse(activity PlanningActivity) PlanningActivityResponse {
	return PlanningActivityResponse{
		ID:          activity.ID,
		PlanningID:  activity.PlanningID,
		ActorUserID: activity.ActorUserID,
		EventType:   activity.EventType,
		EntityType:  activity.EntityType,
		EntityID:    activity.EntityID,
		Metadata:    activity.Metadata,
		CreatedAt:   activity.CreatedAt,
	}
}

func NewPlanningDashboardActivityResponse(activity PlanningDashboardActivity) PlanningDashboardActivityResponse {
	response := PlanningDashboardActivityResponse{
		PlanningID:         activity.PlanningID,
		UpcomingActivities: make([]PlanningUpcomingActivityResponse, 0, len(activity.UpcomingActivities)),
	}

	for _, upcomingActivity := range activity.UpcomingActivities {
		response.UpcomingActivities = append(response.UpcomingActivities, PlanningUpcomingActivityResponse{
			ID:        upcomingActivity.ID,
			Title:     upcomingActivity.Title,
			StartsAt:  upcomingActivity.StartsAt,
			KindLabel: upcomingActivity.KindLabel,
			ImageURL:  upcomingActivity.ImageURL,
		})
	}

	if activity.NextStop != nil {
		response.NextStop = &PlanningNextStopResponse{
			ID:          activity.NextStop.ID,
			Title:       activity.NextStop.Title,
			TimeLabel:   activity.NextStop.TimeLabel,
			Location:    activity.NextStop.Location,
			Description: activity.NextStop.Description,
			ImageURL:    activity.NextStop.ImageURL,
		}
	}

	return response
}

func NewPlanningSummaryResponse(summary PlanningSummary) PlanningSummaryResponse {
	response := PlanningSummaryResponse{
		PlanningID: summary.PlanningID,
		Shortcuts: PlanningSummaryShortcutsResponse{
			FlightsCount:  summary.Shortcuts.FlightsCount,
			BookingsCount: summary.Shortcuts.BookingsCount,
			ExpensesCount: summary.Shortcuts.ExpensesCount,
		},
	}

	if summary.Budget != nil {
		response.Budget = &PlanningSummaryBudgetResponse{
			Target:    summary.Budget.Target,
			Confirmed: summary.Budget.Confirmed,
			Pending:   summary.Budget.Pending,
		}
	}

	return response
}

func NewPlanningItineraryResponse(itinerary PlanningItinerary) PlanningItineraryResponse {
	response := PlanningItineraryResponse{
		PlanningID:   itinerary.PlanningID,
		PlanningName: itinerary.PlanningName,
		Days:         make([]PlanningItineraryDayResponse, 0, len(itinerary.Days)),
	}

	for _, day := range itinerary.Days {
		dayResponse := PlanningItineraryDayResponse{
			ID:        day.ID,
			Label:     day.Label,
			DateLabel: day.DateLabel,
			Stops:     make([]PlanningItineraryStopResponse, 0, len(day.Stops)),
		}

		for _, stop := range day.Stops {
			dayResponse.Stops = append(dayResponse.Stops, PlanningItineraryStopResponse{
				ID:              stop.ID,
				Title:           stop.Title,
				Description:     stop.Description,
				Address:         stop.Address,
				TimeLabel:       stop.TimeLabel,
				DurationMinutes: stop.DurationMinutes,
				Notes:           stop.Notes,
				CostEstimate:    stop.CostEstimate,
				Currency:        stop.Currency,
				ExternalURL:     stop.ExternalURL,
				ImageURL:        stop.ImageURL,
				Status:          stop.Status,
				Category:        stop.Category,
				IsOptional:      stop.IsOptional,
				Coordinates: PlanningItineraryCoordinatesResponse{
					Latitude:  stop.Coordinates.Latitude,
					Longitude: stop.Coordinates.Longitude,
				},
				Bookings: toPlanningRouteBookingResponses(stop.Bookings),
			})
		}

		response.Days = append(response.Days, dayResponse)
	}

	return response
}

func toPlanningItineraryDayResponse(day PlanningItineraryDay) PlanningItineraryDayResponse {
	response := PlanningItineraryDayResponse{
		ID:        day.ID,
		Label:     day.Label,
		DateLabel: day.DateLabel,
		Stops:     make([]PlanningItineraryStopResponse, 0, len(day.Stops)),
	}
	for _, stop := range day.Stops {
		response.Stops = append(response.Stops, toPlanningItineraryStopResponse(stop))
	}
	return response
}

func toPlanningRouteBookingResponses(bookings []PlanningRouteBooking) []PlanningRouteBookingResponse {
	result := make([]PlanningRouteBookingResponse, 0, len(bookings))
	for _, booking := range bookings {
		result = append(result, PlanningRouteBookingResponse{ID: booking.ID, Title: booking.Title, Type: booking.Type, Status: booking.Status, OccursAt: booking.OccursAt})
	}
	return result
}
