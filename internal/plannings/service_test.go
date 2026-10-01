package plannings

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestServiceUpdatePlanningRequiresManagerRole(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	planningID := uuid.New()
	repo := &fakeRepository{
		members: map[uuid.UUID]PlanningMember{
			actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: RoleMember},
		},
	}
	service := NewService(repo)

	_, err := service.UpdatePlanning(context.Background(), actorUserID, planningID, UpdatePlanningInput{Name: stringPointer("Updated")})

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.updatePlanningCalls)
}

func TestServiceDeletePlanningRequiresOwnerRole(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	planningID := uuid.New()
	repo := &fakeRepository{
		planning: Planning{ID: planningID},
		members: map[uuid.UUID]PlanningMember{
			actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: RoleAdmin},
		},
	}
	service := NewService(repo)

	err := service.DeletePlanning(context.Background(), actorUserID, planningID)

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.deletePlanningCalls)
}

func TestServiceDeletePlanningDeletesAsOwner(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	planningID := uuid.New()
	repo := &fakeRepository{
		planning: Planning{ID: planningID},
		members: map[uuid.UUID]PlanningMember{
			actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: RoleOwner},
		},
	}
	service := NewService(repo)

	err := service.DeletePlanning(context.Background(), actorUserID, planningID)

	require.NoError(t, err)
	require.Equal(t, 1, repo.deletePlanningCalls)
}

func TestServiceAddMemberAllowsAdmin(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	targetUserID := uuid.New()
	planningID := uuid.New()
	repo := &fakeRepository{
		members: map[uuid.UUID]PlanningMember{
			actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: RoleAdmin},
		},
	}
	service := NewService(repo)

	member, err := service.AddMember(context.Background(), actorUserID, planningID, AddPlanningMemberInput{UserID: targetUserID, Role: RoleMember})

	require.NoError(t, err)
	require.Equal(t, targetUserID, member.UserID)
	require.Equal(t, 1, repo.addMemberCalls)
}

func TestServiceUpdateMemberRoleBlocksOwnerMutationForAdmin(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	targetUserID := uuid.New()
	planningID := uuid.New()
	repo := &fakeRepository{
		members: map[uuid.UUID]PlanningMember{
			actorUserID:  {PlanningID: planningID, UserID: actorUserID, Role: RoleAdmin},
			targetUserID: {PlanningID: planningID, UserID: targetUserID, Role: RoleOwner},
		},
	}
	service := NewService(repo)

	_, err := service.UpdateMemberRole(context.Background(), actorUserID, planningID, targetUserID, UpdatePlanningMemberRoleInput{Role: RoleMember})

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.updateMemberRoleCalls)
}

func TestServiceRemoveMemberBlocksOwnerRemoval(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	targetUserID := uuid.New()
	planningID := uuid.New()
	repo := &fakeRepository{
		members: map[uuid.UUID]PlanningMember{
			actorUserID:  {PlanningID: planningID, UserID: actorUserID, Role: RoleOwner},
			targetUserID: {PlanningID: planningID, UserID: targetUserID, Role: RoleOwner},
		},
	}
	service := NewService(repo)

	err := service.RemoveMember(context.Background(), actorUserID, planningID, targetUserID)

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.removeMemberCalls)
}

func TestServiceAddMemberRequiresExistingUserEmail(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	planningID := uuid.New()
	repo := &fakeRepository{
		members: map[uuid.UUID]PlanningMember{
			actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: RoleOwner},
		},
		emailLookup: map[string]uuid.UUID{},
	}
	service := NewService(repo)

	_, err := service.AddMember(context.Background(), actorUserID, planningID, AddPlanningMemberInput{Email: "ghost@example.com", Role: RoleMember})

	require.ErrorIs(t, err, ErrUserNotFound)
	require.Zero(t, repo.addMemberCalls)
}

type fakeRepository struct {
	planning               Planning
	planningsResult        PageResult[Planning]
	activityResult         PageResult[PlanningActivity]
	summary                PlanningSummary
	dashboardActivity      PlanningDashboardActivity
	members                map[uuid.UUID]PlanningMember
	emailLookup            map[string]uuid.UUID
	updatePlanningCalls    int
	addMemberCalls         int
	updateMemberRoleCalls  int
	removeMemberCalls      int
	setPlanningArchiveCall int
	deletePlanningCalls    int
}

func (r *fakeRepository) CreatePlanning(_ context.Context, actorUserID uuid.UUID, input CreatePlanningInput) (Planning, error) {
	if r.planning.ID == uuid.Nil {
		r.planning = Planning{
			ID:          uuid.New(),
			GroupID:     input.GroupID,
			OwnerUserID: actorUserID,
			Name:        input.Name,
			Status:      StatusDraft,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
	}
	return r.planning, nil
}

func (r *fakeRepository) GetPlanningByID(_ context.Context, planningID uuid.UUID) (Planning, error) {
	if r.planning.ID == planningID {
		return r.planning, nil
	}
	return Planning{}, ErrPlanningNotFound
}

func (r *fakeRepository) GetSummary(_ context.Context, planningID uuid.UUID) (PlanningSummary, error) {
	if r.summary.PlanningID == planningID {
		return r.summary, nil
	}
	return PlanningSummary{}, ErrPlanningNotFound
}

func (r *fakeRepository) GetDashboardActivity(_ context.Context, planningID uuid.UUID) (PlanningDashboardActivity, error) {
	if r.dashboardActivity.PlanningID == planningID {
		return r.dashboardActivity, nil
	}
	return PlanningDashboardActivity{}, ErrPlanningNotFound
}

func (r *fakeRepository) ListPlannings(_ context.Context, filters PlanningFilters) (PageResult[Planning], error) {
	_ = filters
	return r.planningsResult, nil
}

func (r *fakeRepository) GetMember(_ context.Context, planningID uuid.UUID, userID uuid.UUID) (PlanningMember, error) {
	member, ok := r.members[userID]
	if !ok || member.PlanningID != planningID {
		return PlanningMember{}, ErrMemberNotFound
	}
	return member, nil
}

func (r *fakeRepository) EnsureOwnerMembership(_ context.Context, planningID uuid.UUID, userID uuid.UUID) (PlanningMember, error) {
	if r.planning.ID != planningID || r.planning.OwnerUserID != userID {
		return PlanningMember{}, ErrMemberNotFound
	}
	member := PlanningMember{ID: uuid.New(), PlanningID: planningID, UserID: userID, Role: RoleOwner}
	r.members[userID] = member
	return member, nil
}

func (r *fakeRepository) GetUserIDByEmail(_ context.Context, email string) (uuid.UUID, error) {
	if r.emailLookup == nil {
		return uuid.Nil, ErrUserNotFound
	}

	userID, ok := r.emailLookup[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return uuid.Nil, ErrUserNotFound
	}

	return userID, nil
}

func (r *fakeRepository) ListMembers(_ context.Context, planningID uuid.UUID) ([]PlanningMember, error) {
	result := make([]PlanningMember, 0)
	for _, member := range r.members {
		if member.PlanningID == planningID {
			result = append(result, member)
		}
	}
	return result, nil
}

func (r *fakeRepository) UpdatePlanning(_ context.Context, _ uuid.UUID, planningID uuid.UUID, input UpdatePlanningInput) (Planning, error) {
	r.updatePlanningCalls++
	if r.planning.ID != planningID {
		return Planning{}, ErrPlanningNotFound
	}
	if input.Name != nil {
		r.planning.Name = *input.Name
	}
	return r.planning, nil
}

func (r *fakeRepository) SetPlanningArchived(_ context.Context, _ uuid.UUID, planningID uuid.UUID, archived bool) (Planning, error) {
	r.setPlanningArchiveCall++
	if r.planning.ID != planningID {
		return Planning{}, ErrPlanningNotFound
	}
	r.planning.IsArchived = archived
	return r.planning, nil
}

func (r *fakeRepository) DeletePlanning(_ context.Context, planningID uuid.UUID) error {
	r.deletePlanningCalls++
	if r.planning.ID != planningID {
		return ErrPlanningNotFound
	}
	return nil
}

func (r *fakeRepository) AddMember(_ context.Context, _ uuid.UUID, planningID uuid.UUID, input AddPlanningMemberInput) (PlanningMember, error) {
	r.addMemberCalls++
	if _, ok := r.members[input.UserID]; ok {
		return PlanningMember{}, ErrConflict
	}
	member := PlanningMember{ID: uuid.New(), PlanningID: planningID, UserID: input.UserID, Role: input.Role}
	r.members[input.UserID] = member
	return member, nil
}

func (r *fakeRepository) UpdateMemberRole(_ context.Context, _ uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID, role string) (PlanningMember, error) {
	r.updateMemberRoleCalls++
	member, ok := r.members[targetUserID]
	if !ok || member.PlanningID != planningID {
		return PlanningMember{}, ErrMemberNotFound
	}
	member.Role = role
	r.members[targetUserID] = member
	return member, nil
}

func (r *fakeRepository) RemoveMember(_ context.Context, _ uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID) error {
	r.removeMemberCalls++
	member, ok := r.members[targetUserID]
	if !ok || member.PlanningID != planningID {
		return ErrMemberNotFound
	}
	delete(r.members, targetUserID)
	return nil
}

func (r *fakeRepository) ListActivity(_ context.Context, _ uuid.UUID, page Page) (PageResult[PlanningActivity], error) {
	if page.Size == 0 {
		return PageResult[PlanningActivity]{}, errors.New("page size required")
	}
	return r.activityResult, nil
}

func (r *fakeRepository) GetPlanningItinerary(_ context.Context, planningID uuid.UUID) (PlanningItinerary, error) {
	if r.planning.ID == planningID {
		return PlanningItinerary{PlanningID: planningID, PlanningName: r.planning.Name}, nil
	}
	return PlanningItinerary{}, ErrPlanningNotFound
}

func (r *fakeRepository) CreatePlanningRouteDay(_ context.Context, planningID uuid.UUID, input CreatePlanningRouteDayInput) (PlanningItineraryDay, error) {
	return PlanningItineraryDay{ID: uuid.New(), Label: input.Label, DateLabel: input.DateLabel}, nil
}

func (r *fakeRepository) DeletePlanningRouteDay(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (r *fakeRepository) GetPlanningRouteByID(_ context.Context, planningID uuid.UUID, routeID uuid.UUID) (PlanningItineraryStop, error) {
	if r.planning.ID == planningID {
		return PlanningItineraryStop{ID: routeID}, nil
	}
	return PlanningItineraryStop{}, ErrPlanningNotFound
}

func (r *fakeRepository) CreatePlanningRouteStop(_ context.Context, planningID uuid.UUID, input CreatePlanningRouteStopInput) (PlanningItineraryStop, error) {
	if r.planning.ID != planningID {
		return PlanningItineraryStop{}, ErrPlanningNotFound
	}

	return PlanningItineraryStop{ID: uuid.New(), Title: input.Title}, nil
}

func (r *fakeRepository) UpdatePlanningRouteStop(_ context.Context, planningID uuid.UUID, routeID uuid.UUID, input UpdatePlanningRouteStopInput) (PlanningItineraryStop, error) {
	if r.planning.ID != planningID {
		return PlanningItineraryStop{}, ErrPlanningNotFound
	}

	title := ""
	if input.Title != nil {
		title = *input.Title
	}

	return PlanningItineraryStop{ID: routeID, Title: title}, nil
}

func (r *fakeRepository) DeletePlanningRouteStop(_ context.Context, planningID uuid.UUID, routeID uuid.UUID) error {
	if r.planning.ID != planningID || routeID == uuid.Nil {
		return ErrPlanningNotFound
	}

	return nil
}

func (r *fakeRepository) ReorderPlanningRouteStops(_ context.Context, planningID uuid.UUID, dayID uuid.UUID, stopIDs []uuid.UUID) error {
	if r.planning.ID != planningID || dayID == uuid.Nil || len(stopIDs) == 0 {
		return ErrPlanningNotFound
	}

	return nil
}

func stringPointer(value string) *string {
	return &value
}
