package checklists

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/eleventravel/eleventravel-api/internal/plannings"
)

// PlanningMembership exposes the existing planning membership/role lookup so
// checklists can reuse the planning authorization system instead of creating
// a parallel permissions model.
type PlanningMembership interface {
	GetMember(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error)
}

type PlanningMembersLister interface {
	ListMembers(ctx context.Context, planningID uuid.UUID) ([]plannings.PlanningMember, error)
}

type Service struct {
	repository Repository
	membership PlanningMembership
}

func NewService(repository Repository, membership PlanningMembership) *Service {
	return &Service{repository: repository, membership: membership}
}

func (s *Service) CreateTask(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateChecklistTaskInput) (ChecklistTask, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return ChecklistTask{}, err
	}

	if err := s.validateAssignedUserIDs(ctx, planningID, input.AssignedUserIDs); err != nil {
		return ChecklistTask{}, err
	}

	return s.repository.CreateTask(ctx, planningID, actorUserID, input)
}

func (s *Service) GetTask(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, taskID uuid.UUID) (ChecklistTask, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return ChecklistTask{}, err
	}

	return s.repository.GetTaskByID(ctx, planningID, taskID)
}

func (s *Service) ListTasks(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, filters ChecklistTaskFilters) ([]ChecklistTask, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return nil, err
	}

	return s.repository.ListTasks(ctx, planningID, filters)
}

func (s *Service) UpdateTask(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, taskID uuid.UUID, input UpdateChecklistTaskInput) (ChecklistTask, error) {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return ChecklistTask{}, err
	}

	if input.AssignedUserIDs != nil {
		if err := s.validateAssignedUserIDs(ctx, planningID, *input.AssignedUserIDs); err != nil {
			return ChecklistTask{}, err
		}
	}

	return s.repository.UpdateTask(ctx, planningID, taskID, input)
}

func (s *Service) DeleteTask(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, taskID uuid.UUID) error {
	if _, err := s.requireManagementRole(ctx, planningID, actorUserID); err != nil {
		return err
	}

	return s.repository.DeleteTask(ctx, planningID, taskID)
}

// ToggleCompletion marks a task done or pending for the current user. Group
// tasks are shared: one member's mark completes it for every assignee.
// Individual tasks only toggle the acting member's own completion.
func (s *Service) ToggleCompletion(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, taskID uuid.UUID) (ChecklistTask, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return ChecklistTask{}, err
	}

	task, err := s.repository.GetTaskByID(ctx, planningID, taskID)
	if err != nil {
		return ChecklistTask{}, err
	}

	if len(task.AssignedUserIDs) > 0 && !containsUUID(task.AssignedUserIDs, actorUserID) {
		return ChecklistTask{}, ErrForbidden
	}

	if task.AssignmentMode == AssignmentModeGroup {
		if len(task.CompletedByUserIDs) > 0 {
			if err := s.repository.ClearCompletions(ctx, taskID); err != nil {
				return ChecklistTask{}, err
			}
		} else if err := s.repository.AddCompletion(ctx, taskID, actorUserID); err != nil {
			return ChecklistTask{}, err
		}
	} else {
		if containsUUID(task.CompletedByUserIDs, actorUserID) {
			if err := s.repository.RemoveCompletion(ctx, taskID, actorUserID); err != nil {
				return ChecklistTask{}, err
			}
		} else if err := s.repository.AddCompletion(ctx, taskID, actorUserID); err != nil {
			return ChecklistTask{}, err
		}
	}

	return s.repository.GetTaskByID(ctx, planningID, taskID)
}

func (s *Service) validateAssignedUserIDs(ctx context.Context, planningID uuid.UUID, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}

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

	for _, userID := range userIDs {
		if _, exists := allowed[userID]; !exists {
			return ValidationError{Details: []response.ErrorDetail{{Field: "assigned_user_ids", Message: "assigned_user_ids must belong to the planning members"}}}
		}
	}

	return nil
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

func (s *Service) requireManagementRole(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID) (plannings.PlanningMember, error) {
	member, err := s.requireMembership(ctx, planningID, actorUserID)
	if err != nil {
		return plannings.PlanningMember{}, err
	}

	if member.Role != plannings.RoleOwner && member.Role != plannings.RoleAdmin {
		return plannings.PlanningMember{}, ErrForbidden
	}

	return member, nil
}

func containsUUID(ids []uuid.UUID, target uuid.UUID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}

	return false
}
