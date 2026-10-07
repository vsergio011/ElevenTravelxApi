package expenses

import "time"

type CreateExpenseRequest struct {
	Title              string   `json:"title"`
	AmountCents        int64    `json:"amount_cents"`
	PaidByUserID       string   `json:"paid_by_user_id"`
	Distribution       string   `json:"distribution"`
	ParticipantUserIDs []string `json:"participant_user_ids"`
	Category           *string  `json:"category"`
	Notes              *string  `json:"notes"`
	SpentAt            *string  `json:"spent_at"`
}

type UpdateExpenseRequest struct {
	Title              *string   `json:"title"`
	AmountCents        *int64    `json:"amount_cents"`
	PaidByUserID       *string   `json:"paid_by_user_id"`
	Distribution       *string   `json:"distribution"`
	ParticipantUserIDs *[]string `json:"participant_user_ids"`
	Category           *string   `json:"category"`
	Notes              *string   `json:"notes"`
	SpentAt            *string   `json:"spent_at"`
}

type ExpenseResponse struct {
	ID                 string    `json:"id"`
	PlanningID         string    `json:"planning_id"`
	CreatedByUserID    string    `json:"created_by_user_id"`
	PaidByUserID       string    `json:"paid_by_user_id"`
	Title              string    `json:"title"`
	AmountCents        int64     `json:"amount_cents"`
	Distribution       string    `json:"distribution"`
	ParticipantUserIDs []string  `json:"participant_user_ids"`
	Category           *string   `json:"category"`
	Notes              *string   `json:"notes"`
	SpentAt            time.Time `json:"spent_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func NewExpenseResponse(expense Expense) ExpenseResponse {
	participants := make([]string, 0, len(expense.ParticipantUserIDs))
	for _, id := range expense.ParticipantUserIDs {
		participants = append(participants, id.String())
	}
	return ExpenseResponse{
		ID:                 expense.ID.String(),
		PlanningID:         expense.PlanningID.String(),
		CreatedByUserID:    expense.CreatedByUserID.String(),
		PaidByUserID:       expense.PaidByUserID.String(),
		Title:              expense.Title,
		AmountCents:        expense.AmountCents,
		Distribution:       expense.Distribution,
		ParticipantUserIDs: participants,
		Category:           expense.Category,
		Notes:              expense.Notes,
		SpentAt:            expense.SpentAt,
		CreatedAt:          expense.CreatedAt,
		UpdatedAt:          expense.UpdatedAt,
	}
}

type ShareResponse struct {
	UserID      string `json:"user_id"`
	AmountCents int64  `json:"amount_cents"`
}

type BudgetItemResponse struct {
	ID           string          `json:"id"`
	Source       string          `json:"source"`
	SourceID     string          `json:"source_id"`
	Title        string          `json:"title"`
	Subtitle     *string         `json:"subtitle"`
	Status       string          `json:"status"`
	AmountCents  int64           `json:"amount_cents"`
	Distribution string          `json:"distribution"`
	Payers       []ShareResponse `json:"payers"`
	Shares       []ShareResponse `json:"shares"`
	Date         time.Time       `json:"date"`
}

type BudgetTotalsResponse struct {
	TotalCents     int64 `json:"total_cents"`
	ConfirmedCents int64 `json:"confirmed_cents"`
	PendingCents   int64 `json:"pending_cents"`
	ExpensesCents  int64 `json:"expenses_cents"`
	BookingsCents  int64 `json:"bookings_cents"`
	FlightsCents   int64 `json:"flights_cents"`
	ExpensesCount  int   `json:"expenses_count"`
	BookingsCount  int   `json:"bookings_count"`
	FlightsCount   int   `json:"flights_count"`
}

type MemberBalanceResponse struct {
	UserID       string `json:"user_id"`
	PaidCents    int64  `json:"paid_cents"`
	OwedCents    int64  `json:"owed_cents"`
	BalanceCents int64  `json:"balance_cents"`
}

type SettlementResponse struct {
	FromUserID  string `json:"from_user_id"`
	ToUserID    string `json:"to_user_id"`
	AmountCents int64  `json:"amount_cents"`
}

type BudgetResponse struct {
	Currency    string                  `json:"currency"`
	Items       []BudgetItemResponse    `json:"items"`
	Totals      BudgetTotalsResponse    `json:"totals"`
	Members     []MemberBalanceResponse `json:"members"`
	Settlements []SettlementResponse    `json:"settlements"`
}

func newShareResponses(shares []Share) []ShareResponse {
	result := make([]ShareResponse, 0, len(shares))
	for _, share := range shares {
		result = append(result, ShareResponse{UserID: share.UserID.String(), AmountCents: share.Cents})
	}
	return result
}

func NewBudgetResponse(budget Budget) BudgetResponse {
	items := make([]BudgetItemResponse, 0, len(budget.Items))
	for _, item := range budget.Items {
		items = append(items, BudgetItemResponse{
			ID: item.ID, Source: item.Source, SourceID: item.SourceID.String(), Title: item.Title,
			Subtitle: item.Subtitle, Status: item.Status, AmountCents: item.AmountCents,
			Distribution: item.Distribution, Payers: newShareResponses(item.Payers),
			Shares: newShareResponses(item.Shares), Date: item.Date,
		})
	}
	members := make([]MemberBalanceResponse, 0, len(budget.Members))
	for _, member := range budget.Members {
		members = append(members, MemberBalanceResponse{
			UserID: member.UserID.String(), PaidCents: member.PaidCents,
			OwedCents: member.OwedCents, BalanceCents: member.BalanceCents,
		})
	}
	settlements := make([]SettlementResponse, 0, len(budget.Settlements))
	for _, settlement := range budget.Settlements {
		settlements = append(settlements, SettlementResponse{
			FromUserID: settlement.FromUserID.String(), ToUserID: settlement.ToUserID.String(), AmountCents: settlement.AmountCents,
		})
	}
	totals := budget.Totals
	return BudgetResponse{
		Currency: budget.Currency,
		Items:    items,
		Totals: BudgetTotalsResponse{
			TotalCents: totals.TotalCents, ConfirmedCents: totals.ConfirmedCents, PendingCents: totals.PendingCents,
			ExpensesCents: totals.ExpensesCents, BookingsCents: totals.BookingsCents, FlightsCents: totals.FlightsCents,
			ExpensesCount: totals.ExpensesCount, BookingsCount: totals.BookingsCount, FlightsCount: totals.FlightsCount,
		},
		Members:     members,
		Settlements: settlements,
	}
}
