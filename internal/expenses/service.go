package expenses

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/bookings"
	"github.com/eleventravel/eleventravel-api/internal/flights"
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

type BookingSource interface {
	ListBookings(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, filters bookings.BookingFilters) (bookings.PageResult[bookings.Booking], error)
}

type FlightSource interface {
	List(ctx context.Context, actor, planning uuid.UUID, filters flights.Filters) ([]flights.Flight, error)
}

type Service struct {
	repository Repository
	membership PlanningMembership
	bookings   BookingSource
	flights    FlightSource
}

func NewService(repository Repository, membership PlanningMembership, bookingSource BookingSource, flightSource FlightSource) *Service {
	return &Service{repository: repository, membership: membership, bookings: bookingSource, flights: flightSource}
}

func (s *Service) CreateExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateExpenseInput) (Expense, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Expense{}, err
	}
	if err := s.validateUsers(ctx, planningID, append([]uuid.UUID{input.PaidByUserID}, input.ParticipantUserIDs...)); err != nil {
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

	distribution := expense.Distribution
	if input.Distribution != nil {
		distribution = *input.Distribution
	}
	participants := expense.ParticipantUserIDs
	if input.ParticipantUserIDs != nil {
		participants = *input.ParticipantUserIDs
	}
	if distribution == DistributionGroup {
		empty := []uuid.UUID{}
		input.ParticipantUserIDs = &empty
		participants = empty
	} else if len(participants) == 0 {
		return Expense{}, ValidationError{Details: []response.ErrorDetail{{Field: "participant_user_ids", Message: "select at least one participant"}}}
	}

	toCheck := append([]uuid.UUID{}, participants...)
	if input.PaidByUserID != nil {
		toCheck = append(toCheck, *input.PaidByUserID)
	}
	if err := s.validateUsers(ctx, planningID, toCheck); err != nil {
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

// GetBudget builds one read model from manual expenses plus confirmed/proposed bookings and flights.
// Booking and flight rows are calculated, never copied, so edits stay in their own modules.
func (s *Service) GetBudget(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Budget, error) {
	if _, err := s.requireMembership(ctx, planningID, actorUserID); err != nil {
		return Budget{}, err
	}
	lister, ok := s.membership.(PlanningMembersLister)
	if !ok {
		return Budget{}, ErrForbidden
	}
	members, err := lister.ListMembers(ctx, planningID)
	if err != nil {
		return Budget{}, err
	}
	memberIDs := make([]uuid.UUID, 0, len(members))
	for _, member := range members {
		memberIDs = append(memberIDs, member.UserID)
	}

	items := []BudgetItem{}

	expenseList, err := s.repository.List(ctx, planningID)
	if err != nil {
		return Budget{}, err
	}
	for _, expense := range expenseList {
		items = append(items, BudgetItem{
			ID: "expense:" + expense.ID.String(), Source: SourceExpense, SourceID: expense.ID,
			Title: expense.Title, Subtitle: expense.Category, Status: ItemConfirmed,
			AmountCents: expense.AmountCents, Distribution: expense.Distribution,
			Payers: []Share{{UserID: expense.PaidByUserID, Cents: expense.AmountCents}},
			Shares: computeShares(expense.Distribution, expense.AmountCents, memberIDs, expense.ParticipantUserIDs),
			Date:   expense.SpentAt,
		})
	}

	if s.bookings != nil {
		bookingItems, bookingErr := s.bookingItems(ctx, actorUserID, planningID, memberIDs)
		if bookingErr != nil {
			return Budget{}, bookingErr
		}
		items = append(items, bookingItems...)
	}

	if s.flights != nil {
		flightList, flightErr := s.flights.List(ctx, actorUserID, planningID, flights.Filters{})
		if flightErr != nil {
			return Budget{}, flightErr
		}
		for _, flight := range flightList {
			if flight.CostCents <= 0 {
				continue
			}
			status := ItemPending
			if flight.Status == flights.StatusConfirmed {
				status = ItemConfirmed
			}
			items = append(items, BudgetItem{
				ID: "flight:" + flight.ID.String(), Source: SourceFlight, SourceID: flight.ID,
				Title: flightTitle(flight), Status: status, AmountCents: flight.CostCents,
				Distribution: flight.CostDistribution, Payers: []Share{},
				Shares: computeShares(flight.CostDistribution, flight.CostCents, memberIDs, flight.ParticipantUserIDs),
				Date:   flightDate(flight),
			})
		}
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].Date.After(items[j].Date) })
	return buildBudget(items, memberIDs), nil
}

func (s *Service) bookingItems(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, memberIDs []uuid.UUID) ([]BudgetItem, error) {
	items := []BudgetItem{}
	for page := 1; ; page++ {
		result, err := s.bookings.ListBookings(ctx, actorUserID, planningID, bookings.BookingFilters{Page: bookings.Page{Number: page, Size: 100}})
		if err != nil {
			return nil, err
		}
		for _, booking := range result.Items {
			if booking.CostCents <= 0 {
				continue
			}
			status := ItemPending
			if booking.Status == bookings.StatusConfirmed {
				status = ItemConfirmed
			}
			payers := make([]Share, 0, len(booking.Payments))
			for _, payment := range booking.Payments {
				payers = append(payers, Share{UserID: payment.UserID, Cents: payment.AmountCents})
			}
			subtitle := booking.Type
			date := booking.CreatedAt
			if booking.OccursAt != nil {
				date = *booking.OccursAt
			}
			items = append(items, BudgetItem{
				ID: "booking:" + booking.ID.String(), Source: SourceBooking, SourceID: booking.ID,
				Title: booking.Title, Subtitle: &subtitle, Status: status, AmountCents: booking.CostCents,
				Distribution: booking.CostDistribution, Payers: payers,
				Shares: computeShares(booking.CostDistribution, booking.CostCents, memberIDs, booking.ParticipantUserIDs),
				Date:   date,
			})
		}
		if page >= result.TotalPages {
			return items, nil
		}
	}
}

func flightTitle(flight flights.Flight) string {
	if len(flight.Segments) > 0 {
		first, last := flight.Segments[0], flight.Segments[len(flight.Segments)-1]
		return first.OriginLabel + " → " + last.DestinationLabel
	}
	if flight.FlightNumber != nil && strings.TrimSpace(*flight.FlightNumber) != "" {
		return "Vuelo " + *flight.FlightNumber
	}
	return "Vuelo"
}

func flightDate(flight flights.Flight) time.Time {
	if len(flight.Segments) > 0 {
		return flight.Segments[0].DepartureAt
	}
	return flight.CreatedAt
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

func (s *Service) validateUsers(ctx context.Context, planningID uuid.UUID, userIDs []uuid.UUID) error {
	lister, ok := s.membership.(PlanningMembersLister)
	if !ok {
		return ErrForbidden
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
			return ValidationError{Details: []response.ErrorDetail{{Field: "participant_user_ids", Message: "all users must be planning members"}}}
		}
	}
	return nil
}
