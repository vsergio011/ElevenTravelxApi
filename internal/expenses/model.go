package expenses

import (
	"time"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/expenseledger"
)

const (
	DistributionGroup                = expenseledger.DistributionGroup
	DistributionSelectedParticipants = expenseledger.DistributionSelected
	DistributionIndividual           = expenseledger.DistributionIndividual
	DistributionCustom               = expenseledger.DistributionCustom
)

type Share = expenseledger.Share
type Payment = expenseledger.Payment

type Expense struct {
	ID              uuid.UUID
	PlanningID      uuid.UUID
	CreatedByUserID uuid.UUID
	// PlanningItemID links the expense to a flight/booking; nil means a manual expense.
	PlanningItemID *uuid.UUID
	ItemType       *string
	// ItemRefID is the id of the linked flight/booking row (used for navigation).
	ItemRefID *uuid.UUID
	// ItemStatus is the status of the linked flight/booking, used to mark the expense pending.
	ItemStatus         *string
	Title              string
	AmountCents        int64
	Currency           string
	Distribution       string
	ParticipantUserIDs []uuid.UUID
	Shares             []Share
	Payments           []Payment
	Category           *string
	Notes              *string
	SpentAt            time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateExpenseInput struct {
	Title              string
	AmountCents        int64
	Currency           string
	PlanningItemID     *uuid.UUID
	Payments           []Payment
	Distribution       string
	ParticipantUserIDs []uuid.UUID
	CustomShares       []Share
	Category           *string
	Notes              *string
	SpentAt            *time.Time
}

type UpdateExpenseInput struct {
	Title              *string
	AmountCents        *int64
	Currency           *string
	Payments           *[]Payment
	Distribution       *string
	ParticipantUserIDs *[]uuid.UUID
	CustomShares       *[]Share
	Category           *string
	Notes              *string
	SpentAt            *time.Time
}

const (
	SourceExpense = "expense"
	SourceBooking = "booking"
	SourceFlight  = "flight"

	ItemConfirmed = "confirmed"
	ItemPending   = "pending"
)

type BudgetItem struct {
	ID            string
	Source        string
	SourceID      uuid.UUID
	Title         string
	Subtitle      *string
	Status        string
	AmountCents   int64
	Currency      string
	Distribution  string
	Payers        []Share
	Shares        []Share
	PaymentStatus string
	Date          time.Time
}

type BudgetTotals struct {
	TotalCents     int64
	ConfirmedCents int64
	PendingCents   int64
	ExpensesCents  int64
	BookingsCents  int64
	FlightsCents   int64
	ExpensesCount  int
	BookingsCount  int
	FlightsCount   int
}

type MemberBalance struct {
	UserID       uuid.UUID
	PaidCents    int64
	OwedCents    int64
	BalanceCents int64
}

type Settlement struct {
	FromUserID  uuid.UUID
	ToUserID    uuid.UUID
	AmountCents int64
}

type Budget struct {
	Currency    string
	Items       []BudgetItem
	Totals      BudgetTotals
	Members     []MemberBalance
	Settlements []Settlement
}
