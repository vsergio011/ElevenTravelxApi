package expenses

import (
	"time"

	"github.com/google/uuid"
)

const (
	DistributionGroup                = "group"
	DistributionSelectedParticipants = "selected_participants"
)

type Expense struct {
	ID                 uuid.UUID
	PlanningID         uuid.UUID
	CreatedByUserID    uuid.UUID
	PaidByUserID       uuid.UUID
	Title              string
	AmountCents        int64
	Distribution       string
	ParticipantUserIDs []uuid.UUID
	Category           *string
	Notes              *string
	SpentAt            time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateExpenseInput struct {
	Title              string
	AmountCents        int64
	PaidByUserID       uuid.UUID
	Distribution       string
	ParticipantUserIDs []uuid.UUID
	Category           *string
	Notes              *string
	SpentAt            *time.Time
}

type UpdateExpenseInput struct {
	Title              *string
	AmountCents        *int64
	PaidByUserID       *uuid.UUID
	Distribution       *string
	ParticipantUserIDs *[]uuid.UUID
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

type Share struct {
	UserID uuid.UUID
	Cents  int64
}

type BudgetItem struct {
	ID           string
	Source       string
	SourceID     uuid.UUID
	Title        string
	Subtitle     *string
	Status       string
	AmountCents  int64
	Distribution string
	Payers       []Share
	Shares       []Share
	Date         time.Time
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
