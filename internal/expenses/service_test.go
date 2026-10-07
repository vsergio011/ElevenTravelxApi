package expenses

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
)

type fakeMembership struct {
	members map[uuid.UUID]plannings.PlanningMember
}

func (f fakeMembership) GetMember(_ context.Context, _ uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error) {
	member, ok := f.members[userID]
	if !ok {
		return plannings.PlanningMember{}, plannings.ErrMemberNotFound
	}
	return member, nil
}

func (f fakeMembership) ListMembers(_ context.Context, _ uuid.UUID) ([]plannings.PlanningMember, error) {
	list := []plannings.PlanningMember{}
	for _, member := range f.members {
		list = append(list, member)
	}
	return list, nil
}

type fakeRepository struct {
	items map[uuid.UUID]Expense
}

func (f *fakeRepository) Create(_ context.Context, planningID uuid.UUID, actor uuid.UUID, input CreateExpenseInput) (Expense, error) {
	expense := Expense{ID: uuid.New(), PlanningID: planningID, CreatedByUserID: actor, PaidByUserID: input.PaidByUserID,
		Title: input.Title, AmountCents: input.AmountCents, Distribution: input.Distribution,
		ParticipantUserIDs: input.ParticipantUserIDs, SpentAt: time.Now()}
	f.items[expense.ID] = expense
	return expense, nil
}

func (f *fakeRepository) Get(_ context.Context, _ uuid.UUID, id uuid.UUID) (Expense, error) {
	expense, ok := f.items[id]
	if !ok {
		return Expense{}, ErrExpenseNotFound
	}
	return expense, nil
}

func (f *fakeRepository) List(_ context.Context, _ uuid.UUID) ([]Expense, error) {
	list := []Expense{}
	for _, expense := range f.items {
		list = append(list, expense)
	}
	return list, nil
}

func (f *fakeRepository) Update(_ context.Context, _ uuid.UUID, id uuid.UUID, input UpdateExpenseInput) (Expense, error) {
	expense := f.items[id]
	if input.Title != nil {
		expense.Title = *input.Title
	}
	f.items[id] = expense
	return expense, nil
}

func (f *fakeRepository) Delete(_ context.Context, _ uuid.UUID, id uuid.UUID) error {
	delete(f.items, id)
	return nil
}

func setup() (*Service, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	owner, member, outsider := uuid.New(), uuid.New(), uuid.New()
	planning := uuid.New()
	membership := fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		owner:  {UserID: owner, PlanningID: planning, Role: plannings.RoleOwner},
		member: {UserID: member, PlanningID: planning, Role: plannings.RoleMember},
	}}
	return NewService(&fakeRepository{items: map[uuid.UUID]Expense{}}, membership, nil, nil), planning, owner, member, outsider
}

func TestComputeSharesSplitsRemainder(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	shares := computeShares(DistributionGroup, 100, []uuid.UUID{a, b, c}, nil)
	var total int64
	for _, share := range shares {
		total += share.Cents
	}
	require.Len(t, shares, 3)
	require.Equal(t, int64(100), total)
}

func TestSuggestSettlementsBalancesDebts(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	settlements := suggestSettlements(map[uuid.UUID]int64{a: 500, b: -500})
	require.Len(t, settlements, 1)
	require.Equal(t, b, settlements[0].FromUserID)
	require.Equal(t, a, settlements[0].ToUserID)
	require.Equal(t, int64(500), settlements[0].AmountCents)
}

func TestBudgetBalancesForGroupExpense(t *testing.T) {
	service, planning, owner, member, _ := setup()
	_, err := service.CreateExpense(context.Background(), owner, planning, CreateExpenseInput{
		Title: "Cena", AmountCents: 1000, PaidByUserID: owner, Distribution: DistributionGroup})
	require.NoError(t, err)

	budget, err := service.GetBudget(context.Background(), member, planning)
	require.NoError(t, err)
	require.Equal(t, int64(1000), budget.Totals.ConfirmedCents)
	balances := map[uuid.UUID]int64{}
	for _, m := range budget.Members {
		balances[m.UserID] = m.BalanceCents
	}
	require.Equal(t, int64(500), balances[owner])
	require.Equal(t, int64(-500), balances[member])
	require.Len(t, budget.Settlements, 1)
}

func TestCreateExpenseRejectsOutsiders(t *testing.T) {
	service, planning, owner, _, outsider := setup()
	_, err := service.CreateExpense(context.Background(), outsider, planning, CreateExpenseInput{
		Title: "x", AmountCents: 100, PaidByUserID: outsider, Distribution: DistributionGroup})
	require.ErrorIs(t, err, ErrForbidden)

	_, err = service.CreateExpense(context.Background(), owner, planning, CreateExpenseInput{
		Title: "x", AmountCents: 100, PaidByUserID: outsider, Distribution: DistributionGroup})
	require.Error(t, err)
	require.ErrorAs(t, err, &ValidationError{})
}

func TestOnlyCreatorOrAdminCanEdit(t *testing.T) {
	service, planning, owner, member, _ := setup()
	expense, err := service.CreateExpense(context.Background(), member, planning, CreateExpenseInput{
		Title: "Taxi", AmountCents: 300, PaidByUserID: member, Distribution: DistributionGroup})
	require.NoError(t, err)

	title := "Taxi aeropuerto"
	_, err = service.UpdateExpense(context.Background(), owner, planning, expense.ID, UpdateExpenseInput{Title: &title})
	require.NoError(t, err)

	other, err := service.CreateExpense(context.Background(), owner, planning, CreateExpenseInput{
		Title: "Hotel", AmountCents: 300, PaidByUserID: owner, Distribution: DistributionGroup})
	require.NoError(t, err)
	require.ErrorIs(t, service.DeleteExpense(context.Background(), member, planning, other.ID), ErrForbidden)
	require.NoError(t, service.DeleteExpense(context.Background(), member, planning, expense.ID))
}
