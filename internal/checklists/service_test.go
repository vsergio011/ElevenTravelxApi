package checklists

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
)

type fakeMembership struct {
	members map[uuid.UUID]plannings.PlanningMember
}

func (m *fakeMembership) GetMember(_ context.Context, planningID uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error) {
	member, ok := m.members[userID]
	if !ok || member.PlanningID != planningID {
		return plannings.PlanningMember{}, plannings.ErrMemberNotFound
	}
	return member, nil
}

func (m *fakeMembership) ListMembers(_ context.Context, planningID uuid.UUID) ([]plannings.PlanningMember, error) {
	members := make([]plannings.PlanningMember, 0)
	for _, member := range m.members {
		if member.PlanningID == planningID {
			members = append(members, member)
		}
	}
	return members, nil
}

type fakeRepository struct {
	tasks map[uuid.UUID]ChecklistTask
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{tasks: map[uuid.UUID]ChecklistTask{}}
}

func (r *fakeRepository) CreateTask(_ context.Context, planningID uuid.UUID, actorUserID uuid.UUID, input CreateChecklistTaskInput) (ChecklistTask, error) {
	task := ChecklistTask{
		ID:              uuid.New(),
		PlanningID:      planningID,
		CreatedByUserID: actorUserID,
		Title:           input.Title,
		Type:            input.Type,
		Priority:        input.Priority,
		AssignmentMode:  input.AssignmentMode,
		AssignedUserIDs: input.AssignedUserIDs,
	}
	r.tasks[task.ID] = task
	return task, nil
}

func (r *fakeRepository) GetTaskByID(_ context.Context, planningID uuid.UUID, taskID uuid.UUID) (ChecklistTask, error) {
	task, ok := r.tasks[taskID]
	if !ok || task.PlanningID != planningID {
		return ChecklistTask{}, ErrTaskNotFound
	}
	return task, nil
}

func (r *fakeRepository) ListTasks(_ context.Context, planningID uuid.UUID, _ ChecklistTaskFilters) ([]ChecklistTask, error) {
	tasks := make([]ChecklistTask, 0)
	for _, task := range r.tasks {
		if task.PlanningID == planningID {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func (r *fakeRepository) UpdateTask(_ context.Context, planningID uuid.UUID, taskID uuid.UUID, input UpdateChecklistTaskInput) (ChecklistTask, error) {
	task, ok := r.tasks[taskID]
	if !ok || task.PlanningID != planningID {
		return ChecklistTask{}, ErrTaskNotFound
	}
	if input.Title != nil {
		task.Title = *input.Title
	}
	if input.AssignedUserIDs != nil {
		task.AssignedUserIDs = *input.AssignedUserIDs
	}
	r.tasks[taskID] = task
	return task, nil
}

func (r *fakeRepository) DeleteTask(_ context.Context, planningID uuid.UUID, taskID uuid.UUID) error {
	task, ok := r.tasks[taskID]
	if !ok || task.PlanningID != planningID {
		return ErrTaskNotFound
	}
	delete(r.tasks, taskID)
	return nil
}

func (r *fakeRepository) AddCompletion(_ context.Context, taskID uuid.UUID, userID uuid.UUID) error {
	task := r.tasks[taskID]
	for _, id := range task.CompletedByUserIDs {
		if id == userID {
			return nil
		}
	}
	task.CompletedByUserIDs = append(task.CompletedByUserIDs, userID)
	r.tasks[taskID] = task
	return nil
}

func (r *fakeRepository) RemoveCompletion(_ context.Context, taskID uuid.UUID, userID uuid.UUID) error {
	task := r.tasks[taskID]
	filtered := make([]uuid.UUID, 0, len(task.CompletedByUserIDs))
	for _, id := range task.CompletedByUserIDs {
		if id != userID {
			filtered = append(filtered, id)
		}
	}
	task.CompletedByUserIDs = filtered
	r.tasks[taskID] = task
	return nil
}

func (r *fakeRepository) ClearCompletions(_ context.Context, taskID uuid.UUID) error {
	task := r.tasks[taskID]
	task.CompletedByUserIDs = nil
	r.tasks[taskID] = task
	return nil
}

func newTestSetup() (*Service, *fakeRepository, *fakeMembership, uuid.UUID, uuid.UUID, uuid.UUID) {
	planningID := uuid.New()
	ownerID := uuid.New()
	memberID := uuid.New()

	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		ownerID:  {PlanningID: planningID, UserID: ownerID, Role: plannings.RoleOwner},
		memberID: {PlanningID: planningID, UserID: memberID, Role: plannings.RoleMember},
	}}
	repository := newFakeRepository()
	service := NewService(repository, membership)

	return service, repository, membership, planningID, ownerID, memberID
}

func TestCreateTaskRequiresManagementRole(t *testing.T) {
	t.Parallel()
	service, _, _, planningID, ownerID, memberID := newTestSetup()

	input := CreateChecklistTaskInput{Title: "Preparar maletas", Type: "equipaje", Priority: PriorityMedium, AssignmentMode: AssignmentModeGroup}

	if _, err := service.CreateTask(context.Background(), memberID, planningID, input); err == nil {
		t.Fatal("expected a member without management role to be forbidden")
	}

	task, err := service.CreateTask(context.Background(), ownerID, planningID, input)
	require.NoError(t, err)
	require.Equal(t, "equipaje", task.Type)
}

func TestToggleCompletionGroupTaskMarksForEveryone(t *testing.T) {
	t.Parallel()
	service, _, _, planningID, ownerID, memberID := newTestSetup()

	task, err := service.CreateTask(context.Background(), ownerID, planningID, CreateChecklistTaskInput{
		Title: "Revisar pasaportes", Type: "documentacion", Priority: PriorityHigh, AssignmentMode: AssignmentModeGroup,
	})
	require.NoError(t, err)

	toggled, err := service.ToggleCompletion(context.Background(), memberID, planningID, task.ID)
	require.NoError(t, err)
	require.Contains(t, toggled.CompletedByUserIDs, memberID)

	toggledAgain, err := service.ToggleCompletion(context.Background(), ownerID, planningID, task.ID)
	require.NoError(t, err)
	require.Empty(t, toggledAgain.CompletedByUserIDs)
}

func TestToggleCompletionIndividualTaskOnlyAffectsActor(t *testing.T) {
	t.Parallel()
	service, _, _, planningID, ownerID, memberID := newTestSetup()

	task, err := service.CreateTask(context.Background(), ownerID, planningID, CreateChecklistTaskInput{
		Title: "Empacar equipaje", Type: "equipaje", Priority: PriorityLow, AssignmentMode: AssignmentModeIndividual,
	})
	require.NoError(t, err)

	if _, err := service.ToggleCompletion(context.Background(), memberID, planningID, task.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	final, err := service.ToggleCompletion(context.Background(), ownerID, planningID, task.ID)
	require.NoError(t, err)
	require.Contains(t, final.CompletedByUserIDs, memberID)
	require.Contains(t, final.CompletedByUserIDs, ownerID)
}
