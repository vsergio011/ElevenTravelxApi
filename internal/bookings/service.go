package bookings

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
)

// PlanningMembership exposes the existing planning membership/role lookup so
// bookings can reuse the planning authorization system instead of creating a
// parallel permissions model.
type PlanningMembership interface {
	GetMember(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error)
}

type PlanningMembersLister interface {
	ListMembers(ctx context.Context, planningID uuid.UUID) ([]plannings.PlanningMember, error)
}

type FlightBudgetSource interface {
	GetConfirmedFlightBudget(ctx context.Context, planningID uuid.UUID, participantIDs []uuid.UUID) (map[uuid.UUID]int64, error)
}

type Service struct {
	repository          Repository
	membership          PlanningMembership
	flightBudgetSources []FlightBudgetSource
}

func NewService(repository Repository, membership PlanningMembership, flightBudgetSources ...FlightBudgetSource) *Service {
	return &Service{repository: repository, membership: membership, flightBudgetSources: flightBudgetSources}
}

func (s *Service) CreateBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateBookingInput) (Booking, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Booking{}, err
	}
	if err := s.validatePlanningUsers(ctx, planningID, input.ParticipantUserIDs, input.Payments); err != nil {
		return Booking{}, err
	}

	return s.repository.CreateBooking(ctx, actorUserID, planningID, input)
}

func (s *Service) GetBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Booking{}, err
	}

	return s.repository.GetBookingByID(ctx, planningID, bookingID)
}

func (s *Service) ListBookings(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, filters BookingFilters) (PageResult[Booking], error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return PageResult[Booking]{}, err
	}

	return s.repository.ListBookings(ctx, planningID, filters)
}

func (s *Service) UpdateBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, bookingID uuid.UUID, input UpdateBookingInput) (Booking, error) {
	if err := s.requireEditPermission(ctx, planningID, actorUserID, bookingID); err != nil {
		return Booking{}, err
	}
	if input.ParticipantUserIDs != nil || input.Payments != nil {
		var participants []uuid.UUID
		if input.ParticipantUserIDs != nil {
			participants = *input.ParticipantUserIDs
		}
		var payments []Payment
		if input.Payments != nil {
			payments = *input.Payments
		}
		if err := s.validatePlanningUsers(ctx, planningID, participants, payments); err != nil {
			return Booking{}, err
		}
	}

	return s.repository.UpdateBooking(ctx, planningID, bookingID, input)
}

func (s *Service) ConfirmBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	booking, err := s.repository.GetBookingByID(ctx, planningID, bookingID)
	if err != nil {
		return Booking{}, err
	}

	if err := s.requireEditPermissionForBooking(ctx, planningID, actorUserID, booking); err != nil {
		return Booking{}, err
	}

	if booking.Status == StatusConfirmed {
		return booking, nil
	}

	return s.repository.ConfirmBooking(ctx, planningID, bookingID)
}

func (s *Service) DeleteBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, bookingID uuid.UUID) error {
	if err := s.requireEditPermission(ctx, planningID, actorUserID, bookingID); err != nil {
		return err
	}

	return s.repository.DeleteBooking(ctx, planningID, bookingID)
}

func (s *Service) GetBudget(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) ([]BudgetLine, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return nil, err
	}
	lister, ok := s.membership.(PlanningMembersLister)
	if !ok {
		return nil, ErrForbidden
	}
	members, err := lister.ListMembers(ctx, planningID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.UserID)
	}
	bookings, err := s.repository.ListBookings(ctx, planningID, BookingFilters{Page: Page{Number: 1, Size: 100}})
	if err != nil {
		return nil, err
	}
	allocations := IndividualBudget(bookings.Items, ids)
	for _, source := range s.flightBudgetSources {
		flightAllocations, sourceErr := source.GetConfirmedFlightBudget(ctx, planningID, ids)
		if sourceErr != nil {
			return nil, sourceErr
		}
		for userID, cents := range flightAllocations {
			allocations[userID] += cents
		}
	}
	result := make([]BudgetLine, 0, len(ids))
	for _, id := range ids {
		result = append(result, BudgetLine{UserID: id, Cents: allocations[id]})
	}
	return result, nil
}

func (s *Service) requireMembership(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID) (plannings.PlanningMember, error) {
	member, err := s.membership.GetMember(ctx, planningID, actorUserID)
	if err != nil {
		if errors.Is(err, plannings.ErrMemberNotFound) {
			return plannings.PlanningMember{}, ErrForbidden
		}

		return plannings.PlanningMember{}, err
	}

	return member, nil
}

func (s *Service) validatePlanningUsers(ctx context.Context, planningID uuid.UUID, participantIDs []uuid.UUID, payments []Payment) error {
	lister, ok := s.membership.(PlanningMembersLister)
	if !ok {
		return nil
	}
	members, err := lister.ListMembers(ctx, planningID)
	if err != nil {
		return err
	}
	allowed := make(map[uuid.UUID]struct{}, len(members))
	for _, member := range members {
		allowed[member.UserID] = struct{}{}
	}
	for _, userID := range participantIDs {
		if _, exists := allowed[userID]; !exists {
			return ErrForbidden
		}
	}
	for _, payment := range payments {
		if _, exists := allowed[payment.UserID]; !exists {
			return ErrForbidden
		}
	}
	return nil
}

// requireEditPermission allows only the booking creator or a planning
// owner/admin to edit or delete a booking.
func (s *Service) requireEditPermission(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, bookingID uuid.UUID) error {
	booking, err := s.repository.GetBookingByID(ctx, planningID, bookingID)
	if err != nil {
		return err
	}

	return s.requireEditPermissionForBooking(ctx, planningID, actorUserID, booking)
}

func (s *Service) requireEditPermissionForBooking(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, booking Booking) error {
	member, err := s.requireMembership(ctx, planningID, actorUserID)
	if err != nil {
		return err
	}

	if member.Role == plannings.RoleOwner || member.Role == plannings.RoleAdmin {
		return nil
	}

	if booking.CreatedByUserID == actorUserID {
		return nil
	}

	return ErrForbidden
}
