package bookings

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
)

// PlanningAccess reuses planning membership and route-stop ownership for
// booking authorization.
type PlanningAccess interface {
	GetMember(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error)
	GetPlanningRouteStopCreator(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID) (uuid.UUID, error)
}

type PlanningMembersLister interface {
	ListMembers(ctx context.Context, planningID uuid.UUID) ([]plannings.PlanningMember, error)
}

type Service struct {
	repository Repository
	membership PlanningAccess
}

func NewService(repository Repository, membership PlanningAccess) *Service {
	return &Service{repository: repository, membership: membership}
}

func (s *Service) CreateBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateBookingInput) (Booking, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Booking{}, err
	}
	if input.RouteStopID != nil {
		if err := s.requireRouteStopAssociationPermission(ctx, planningID, actorUserID, nil, input.RouteStopID); err != nil {
			return Booking{}, err
		}
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
	booking, err := s.repository.GetBookingByID(ctx, planningID, bookingID)
	if err != nil {
		return Booking{}, err
	}
	associationChanged := input.RouteStopIDSet && !sameUUIDPointer(booking.RouteStopID, input.RouteStopID)
	if !associationChanged || !isRouteStopAssociationOnly(input) {
		if err := s.requireEditPermissionForBooking(ctx, planningID, actorUserID, booking); err != nil {
			return Booking{}, err
		}
	}
	if associationChanged {
		if err := s.requireRouteStopAssociationPermission(ctx, planningID, actorUserID, booking.RouteStopID, input.RouteStopID); err != nil {
			return Booking{}, err
		}
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

func (s *Service) requireRouteStopAssociationPermission(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, currentStopID *uuid.UUID, nextStopID *uuid.UUID) error {
	member, err := s.requireMembership(ctx, planningID, actorUserID)
	if err != nil {
		return err
	}

	var nextStopCreator uuid.UUID
	if nextStopID != nil {
		nextStopCreator, err = s.getRouteStopCreator(ctx, planningID, *nextStopID)
		if err != nil {
			return err
		}
	}

	if member.Role == plannings.RoleOwner || member.Role == plannings.RoleAdmin {
		return nil
	}

	if currentStopID != nil && (nextStopID == nil || *currentStopID != *nextStopID) {
		creator, err := s.getRouteStopCreator(ctx, planningID, *currentStopID)
		if err != nil {
			return err
		}
		if creator != actorUserID {
			return ErrForbidden
		}
	}

	if nextStopID != nil && (currentStopID == nil || *currentStopID != *nextStopID) && nextStopCreator != actorUserID {
		return ErrForbidden
	}

	return nil
}

func (s *Service) getRouteStopCreator(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID) (uuid.UUID, error) {
	creatorUserID, err := s.membership.GetPlanningRouteStopCreator(ctx, planningID, routeID)
	if errors.Is(err, plannings.ErrPlanningNotFound) {
		return uuid.Nil, ErrRouteStopNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return creatorUserID, nil
}

func sameUUIDPointer(left *uuid.UUID, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func isRouteStopAssociationOnly(input UpdateBookingInput) bool {
	return input.RouteStopIDSet &&
		input.Title == nil &&
		input.Type == nil &&
		input.Status == nil &&
		input.ExternalURL == nil &&
		input.OccursAt == nil &&
		input.Location == nil &&
		input.Notes == nil &&
		input.CostCents == nil &&
		input.CostDistribution == nil &&
		input.ParticipantUserIDs == nil &&
		input.Payments == nil
}
