package plannings

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreatePlanning(ctx context.Context, actorUserID uuid.UUID, input CreatePlanningInput) (Planning, error) {
	if input.Name == "" {
		return Planning{}, ValidationError{}
	}

	return s.repository.CreatePlanning(ctx, actorUserID, input)
}

func (s *Service) GetPlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Planning{}, err
	}

	return s.repository.GetPlanningByID(ctx, planningID)
}

func (s *Service) GetSummary(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningSummary, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return PlanningSummary{}, err
	}

	return s.repository.GetSummary(ctx, planningID)
}

func (s *Service) GetDashboardActivity(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningDashboardActivity, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return PlanningDashboardActivity{}, err
	}

	return s.repository.GetDashboardActivity(ctx, planningID)
}

func (s *Service) ListPlannings(ctx context.Context, actorUserID uuid.UUID, filters PlanningFilters) (PageResult[Planning], error) {
	filters.OnlyMember = actorUserID
	return s.repository.ListPlannings(ctx, filters)
}

func (s *Service) UpdatePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input UpdatePlanningInput) (Planning, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return Planning{}, err
	}

	return s.repository.UpdatePlanning(ctx, actorUserID, planningID, input)
}

func (s *Service) ArchivePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return Planning{}, err
	}

	return s.repository.SetPlanningArchived(ctx, actorUserID, planningID, true)
}

func (s *Service) UnarchivePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return Planning{}, err
	}

	return s.repository.SetPlanningArchived(ctx, actorUserID, planningID, false)
}

func (s *Service) ListMembers(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) ([]PlanningMember, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return nil, err
	}

	return s.repository.ListMembers(ctx, planningID)
}

func (s *Service) AddMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input AddPlanningMemberInput) (PlanningMember, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return PlanningMember{}, err
	}

	if input.UserID == uuid.Nil {
		userID, err := s.repository.GetUserIDByEmail(ctx, input.Email)
		if err != nil {
			return PlanningMember{}, err
		}
		input.UserID = userID
	}

	if input.UserID == uuid.Nil {
		return PlanningMember{}, ErrUserNotFound
	}

	return s.repository.AddMember(ctx, actorUserID, planningID, input)
}

func (s *Service) UpdateMemberRole(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID, input UpdatePlanningMemberRoleInput) (PlanningMember, error) {
	actorMember, err := s.requireManagementRole(ctx, planningID, actorUserID)
	if err != nil {
		return PlanningMember{}, err
	}

	targetMember, err := s.repository.GetMember(ctx, planningID, targetUserID)
	if err != nil {
		return PlanningMember{}, err
	}

	if targetMember.Role == RoleOwner {
		return PlanningMember{}, ErrForbidden
	}

	if actorMember.Role == RoleAdmin && input.Role == RoleOwner {
		return PlanningMember{}, ErrForbidden
	}

	return s.repository.UpdateMemberRole(ctx, actorUserID, planningID, targetUserID, input.Role)
}

func (s *Service) RemoveMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID) error {
	_, err := s.requireManagementRole(ctx, planningID, actorUserID)
	if err != nil {
		return err
	}

	targetMember, err := s.repository.GetMember(ctx, planningID, targetUserID)
	if err != nil {
		return err
	}

	if targetMember.Role == RoleOwner {
		return ErrForbidden
	}

	return s.repository.RemoveMember(ctx, actorUserID, planningID, targetUserID)
}

func (s *Service) ListActivity(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, page Page) (PageResult[PlanningActivity], error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return PageResult[PlanningActivity]{}, err
	}

	return s.repository.ListActivity(ctx, planningID, page)
}

func (s *Service) GetPlanningItinerary(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningItinerary, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return PlanningItinerary{}, err
	}

	return s.repository.GetPlanningItinerary(ctx, planningID)
}

func (s *Service) CreatePlanningRouteDay(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreatePlanningRouteDayInput) (PlanningItineraryDay, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return PlanningItineraryDay{}, err
	}
	if input.Label == "" || input.DateLabel == "" {
		return PlanningItineraryDay{}, ValidationError{}
	}

	return s.repository.CreatePlanningRouteDay(ctx, planningID, input)
}

func (s *Service) DeletePlanningRouteDay(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, dayID uuid.UUID) error {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return err
	}

	return s.repository.DeletePlanningRouteDay(ctx, planningID, dayID)
}

func (s *Service) GetPlanningRouteByID(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID) (PlanningItineraryStop, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return PlanningItineraryStop{}, err
	}

	return s.repository.GetPlanningRouteByID(ctx, planningID, routeID)
}

func (s *Service) CreatePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreatePlanningRouteStopInput) (PlanningItineraryStop, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return PlanningItineraryStop{}, err
	}

	return s.repository.CreatePlanningRouteStop(ctx, planningID, input)
}

func (s *Service) UpdatePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID, input UpdatePlanningRouteStopInput) (PlanningItineraryStop, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return PlanningItineraryStop{}, err
	}

	return s.repository.UpdatePlanningRouteStop(ctx, planningID, routeID, input)
}

func (s *Service) DeletePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID) error {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return err
	}

	return s.repository.DeletePlanningRouteStop(ctx, planningID, routeID)
}

func (s *Service) ReorderPlanningRouteStops(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input ReorderPlanningRouteStopsInput) error {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return err
	}

	return s.repository.ReorderPlanningRouteStops(ctx, planningID, input.DayID, input.StopIDs)
}

func (s *Service) requireMembership(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID) (PlanningMember, error) {
	member, err := s.repository.GetMember(ctx, planningID, actorUserID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			// owner_user_id is the source of truth for ownership. Recreate the
			// derived membership row if it was deleted outside the application.
			member, repairErr := s.repository.EnsureOwnerMembership(ctx, planningID, actorUserID)
			if repairErr == nil {
				return member, nil
			}
			if errors.Is(repairErr, ErrMemberNotFound) {
				return PlanningMember{}, ErrForbidden
			}
			return PlanningMember{}, repairErr
		}

		return PlanningMember{}, err
	}

	return member, nil
}

func (s *Service) requireManagementRole(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID) (PlanningMember, error) {
	member, err := s.requireMembership(ctx, planningID, actorUserID)
	if err != nil {
		return PlanningMember{}, err
	}

	if member.Role != RoleOwner && member.Role != RoleAdmin {
		return PlanningMember{}, ErrForbidden
	}

	return member, nil
}
