package expenses

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/expenseledger"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/eleventravel/eleventravel-api/internal/plannings"
)

const budgetCurrency = "EUR"

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

func (s *Service) CreateExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateExpenseInput) (Expense, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Expense{}, err
	}
	if err := s.validateEconomics(ctx, planningID, input.AmountCents, input.Distribution, input.ParticipantUserIDs, input.CustomShares, input.Payments); err != nil {
		return Expense{}, err
	}
	return s.repository.Create(ctx, planningID, actorUserID, input)
}

func (s *Service) GetExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, expenseID uuid.UUID) (Expense, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Expense{}, err
	}
	return s.repository.Get(ctx, planningID, expenseID)
}

func (s *Service) ListExpenses(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) ([]Expense, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, planningID)
}

func (s *Service) UpdateExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, expenseID uuid.UUID, input UpdateExpenseInput) (Expense, error) {
	expense, err := s.requireEditPermission(ctx, planningID, actorUserID, expenseID)
	if err != nil {
		return Expense{}, err
	}

	amount := expense.AmountCents
	if input.AmountCents != nil {
		amount = *input.AmountCents
	}
	distribution := expense.Distribution
	if input.Distribution != nil {
		distribution = *input.Distribution
	}
	participants := expense.ParticipantUserIDs
	if input.ParticipantUserIDs != nil {
		participants = *input.ParticipantUserIDs
	}
	custom := expense.Shares
	if input.CustomShares != nil {
		custom = *input.CustomShares
	}
	payments := expense.Payments
	if input.Payments != nil {
		payments = *input.Payments
	}
	if distribution == DistributionGroup {
		empty := []uuid.UUID{}
		input.ParticipantUserIDs = &empty
		participants = empty
	}
	if err := s.validateEconomics(ctx, planningID, amount, distribution, participants, custom, payments); err != nil {
		return Expense{}, err
	}
	return s.repository.Update(ctx, planningID, expenseID, input)
}

func (s *Service) DeleteExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, expenseID uuid.UUID) error {
	if _, err := s.requireEditPermission(ctx, planningID, actorUserID, expenseID); err != nil {
		return err
	}
	return s.repository.Delete(ctx, planningID, expenseID)
}

// GetBudget is a pure read model over expenses: flights and bookings appear through their linked expense,
// so nothing is computed twice.
func (s *Service) GetBudget(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Budget, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Budget{}, err
	}
	memberIDs, err := s.memberIDs(ctx, planningID)
	if err != nil {
		return Budget{}, err
	}
	expenseList, err := s.repository.List(ctx, planningID)
	if err != nil {
		return Budget{}, err
	}

	items := make([]BudgetItem, 0, len(expenseList))
	for _, expense := range expenseList {
		items = append(items, budgetItemFromExpense(expense, memberIDs))
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Date.After(items[j].Date) })
	return buildBudget(items, memberIDs), nil
}

func budgetItemFromExpense(expense Expense, memberIDs []uuid.UUID) BudgetItem {
	source := SourceExpense
	if expense.ItemType != nil {
		source = *expense.ItemType
	}
	status := ItemConfirmed
	if expense.ItemStatus != nil && *expense.ItemStatus != "confirmed" {
		status = ItemPending
	}
	shares := expense.Shares
	if len(shares) == 0 {
		shares = computeShares(expense.Distribution, expense.AmountCents, memberIDs, expense.ParticipantUserIDs)
	}
	payers := make([]Share, 0, len(expense.Payments))
	for _, payment := range expense.Payments {
		payers = append(payers, Share{UserID: payment.UserID, Cents: payment.AmountCents})
	}
	sourceID := expense.ID
	if expense.ItemRefID != nil {
		sourceID = *expense.ItemRefID
	}
	return BudgetItem{
		ID: "expense:" + expense.ID.String(), Source: source, SourceID: sourceID,
		Title: expense.Title, Subtitle: expense.Category, Status: status,
		AmountCents: expense.AmountCents, Currency: expense.Currency, Distribution: expense.Distribution,
		Payers: payers, Shares: shares,
		PaymentStatus: expenseledger.PaymentStatus(expense.AmountCents, expense.Payments),
		Date:          expense.SpentAt,
	}
}

func buildBudget(items []BudgetItem, memberIDs []uuid.UUID) Budget {
	paid := map[uuid.UUID]int64{}
	owed := map[uuid.UUID]int64{}
	balance := map[uuid.UUID]int64{}
	for _, id := range memberIDs {
		balance[id] = 0
	}
	totals := BudgetTotals{}

	for _, item := range items {
		totals.TotalCents += item.AmountCents
		switch item.Source {
		case SourceExpense:
			totals.ExpensesCents += item.AmountCents
			totals.ExpensesCount++
		case SourceBooking:
			totals.BookingsCents += item.AmountCents
			totals.BookingsCount++
		case SourceFlight:
			totals.FlightsCents += item.AmountCents
			totals.FlightsCount++
		}
		if item.Status != ItemConfirmed {
			totals.PendingCents += item.AmountCents
			continue
		}
		totals.ConfirmedCents += item.AmountCents
		for _, share := range item.Shares {
			owed[share.UserID] += share.Cents
		}
		for _, payer := range item.Payers {
			paid[payer.UserID] += payer.Cents
		}
		// Items without recorded payers cannot be settled, so they stay out of balances.
		if len(item.Payers) == 0 {
			continue
		}
		for _, payer := range item.Payers {
			balance[payer.UserID] += payer.Cents
		}
		for _, share := range item.Shares {
			balance[share.UserID] -= share.Cents
		}
	}

	members := make([]MemberBalance, 0, len(memberIDs))
	for _, id := range memberIDs {
		members = append(members, MemberBalance{UserID: id, PaidCents: paid[id], OwedCents: owed[id], BalanceCents: balance[id]})
	}
	return Budget{
		Currency:    budgetCurrency,
		Items:       items,
		Totals:      totals,
		Members:     members,
		Settlements: suggestSettlements(balance),
	}
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

// requireEditPermission allows only the expense creator or a planning owner/admin.
func (s *Service) requireEditPermission(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, expenseID uuid.UUID) (Expense, error) {
	member, err := s.requireMembership(ctx, planningID, actorUserID)
	if err != nil {
		return Expense{}, err
	}
	expense, err := s.repository.Get(ctx, planningID, expenseID)
	if err != nil {
		return Expense{}, err
	}
	if expense.CreatedByUserID != actorUserID && member.Role != plannings.RoleOwner && member.Role != plannings.RoleAdmin {
		return Expense{}, ErrForbidden
	}
	return expense, nil
}

func (s *Service) memberIDs(ctx context.Context, planningID uuid.UUID) ([]uuid.UUID, error) {
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
	return ids, nil
}

// validateEconomics checks that payers, participants and shares are planning members and that the amounts are coherent.
func (s *Service) validateEconomics(ctx context.Context, planningID uuid.UUID, amount int64, distribution string, participants []uuid.UUID, custom []Share, payments []Payment) error {
	memberIDs, err := s.memberIDs(ctx, planningID)
	if err != nil {
		return err
	}
	allowed := make(map[uuid.UUID]struct{}, len(memberIDs))
	for _, id := range memberIDs {
		allowed[id] = struct{}{}
	}
	invalid := func(field, message string) error {
		return ValidationError{Details: []response.ErrorDetail{{Field: field, Message: message}}}
	}
	for _, id := range participants {
		if _, ok := allowed[id]; !ok {
			return invalid("participant_user_ids", "all users must be planning members")
		}
	}
	if distribution == DistributionSelectedParticipants && len(participants) == 0 {
		return invalid("participant_user_ids", "select at least one participant")
	}
	if distribution == DistributionCustom {
		for _, share := range custom {
			if _, ok := allowed[share.UserID]; !ok {
				return invalid("shares", "all users must be planning members")
			}
		}
		if err := expenseledger.ValidateShares(distribution, amount, custom); err != nil {
			return invalid("shares", "shares must add up to amount_cents")
		}
	}
	var paid int64
	for _, payment := range payments {
		if _, ok := allowed[payment.UserID]; !ok {
			return invalid("payments", "all payers must be planning members")
		}
		if payment.AmountCents <= 0 {
			return invalid("payments", "payment amounts must be positive")
		}
		paid += payment.AmountCents
	}
	if paid > amount {
		return invalid("payments", "payments cannot exceed amount_cents")
	}
	return nil
}
