package transports

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eleventravel/eleventravel-api/internal/expenseledger"
	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("transport not found")
	ErrForbidden  = errors.New("forbidden")
	ErrValidation = errors.New("invalid transport")
)

type Payment struct {
	UserID      uuid.UUID `json:"user_id"`
	AmountCents int64     `json:"amount_cents"`
	PaidAt      time.Time `json:"paid_at,omitempty"`
}

type Input struct {
	Status             string      `json:"status"`
	Type               string      `json:"transport_type"`
	Origin             string      `json:"origin"`
	Destination        string      `json:"destination"`
	DepartureDate      *string     `json:"departure_date"`
	DepartureTime      *string     `json:"departure_time"`
	ArrivalDate        *string     `json:"arrival_date"`
	ArrivalTime        *string     `json:"arrival_time"`
	Operator           *string     `json:"operator"`
	Reference          *string     `json:"reference"`
	Notes              *string     `json:"notes"`
	CostCents          int64       `json:"cost_cents"`
	Currency           string      `json:"currency"`
	CostDistribution   string      `json:"cost_distribution"`
	ParticipantUserIDs []uuid.UUID `json:"participant_user_ids"`
	Payments           *[]Payment  `json:"payments"`
}

type Transport struct {
	ID              uuid.UUID `json:"id"`
	PlanningID      uuid.UUID `json:"planning_id"`
	CreatedByUserID uuid.UUID `json:"created_by_user_id"`
	PlanningItemID  uuid.UUID `json:"-"`
	Input
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Repository interface {
	Create(context.Context, uuid.UUID, uuid.UUID, Input) (Transport, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Transport, error)
	List(context.Context, uuid.UUID) ([]Transport, error)
	Update(context.Context, uuid.UUID, uuid.UUID, Input) (Transport, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type Membership interface {
	GetMember(context.Context, uuid.UUID, uuid.UUID) (plannings.PlanningMember, error)
	ListMembers(context.Context, uuid.UUID) ([]plannings.PlanningMember, error)
}

type Service struct {
	repository Repository
	membership Membership
}

func NewService(repository Repository, membership Membership) *Service {
	return &Service{repository: repository, membership: membership}
}

func (s *Service) member(ctx context.Context, planning, actor uuid.UUID) (plannings.PlanningMember, error) {
	member, err := s.membership.GetMember(ctx, planning, actor)
	if errors.Is(err, plannings.ErrMemberNotFound) {
		return member, ErrForbidden
	}
	return member, err
}

func (s *Service) validate(ctx context.Context, planning uuid.UUID, input Input) error {
	if err := validateInput(input); err != nil {
		return err
	}
	members, err := s.membership.ListMembers(ctx, planning)
	if err != nil {
		return err
	}
	allowed := make(map[uuid.UUID]bool, len(members))
	for _, member := range members {
		allowed[member.UserID] = true
	}
	for _, id := range input.ParticipantUserIDs {
		if !allowed[id] {
			return ErrForbidden
		}
	}
	if input.Payments != nil {
		ledgerPayments := make([]expenseledger.Payment, 0, len(*input.Payments))
		for _, payment := range *input.Payments {
			if !allowed[payment.UserID] {
				return ErrForbidden
			}
			ledgerPayments = append(ledgerPayments, expenseledger.Payment{
				UserID: payment.UserID, AmountCents: payment.AmountCents, PaidAt: payment.PaidAt,
			})
		}
		if expenseledger.ValidatePayments(
			expenseledger.EconomicTotal(input.CostDistribution, input.CostCents, len(members)),
			ledgerPayments,
		) != nil {
			return fmt.Errorf("%w: payments exceed the distributed total", ErrValidation)
		}
	}
	return nil
}

func validateInput(input Input) error {
	switch input.Type {
	case "train", "bus", "urban", "ferry", "taxi", "rental_car", "other":
	default:
		return fmt.Errorf("%w: invalid transport type", ErrValidation)
	}
	if input.Status != "draft" && input.Status != "confirmed" {
		return fmt.Errorf("%w: invalid status", ErrValidation)
	}
	if strings.TrimSpace(input.Origin) == "" || strings.TrimSpace(input.Destination) == "" {
		return fmt.Errorf("%w: origin and destination are required", ErrValidation)
	}
	if input.CostCents < 0 || (input.Currency != "EUR" && input.Currency != "USD") {
		return fmt.Errorf("%w: invalid amount or currency", ErrValidation)
	}
	if input.CostDistribution != expenseledger.DistributionGroup && input.CostDistribution != expenseledger.DistributionIndividual && input.CostDistribution != expenseledger.DistributionSelected {
		return fmt.Errorf("%w: invalid distribution", ErrValidation)
	}
	if input.CostDistribution == expenseledger.DistributionSelected && len(input.ParticipantUserIDs) == 0 {
		return fmt.Errorf("%w: select at least one participant", ErrValidation)
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range input.ParticipantUserIDs {
		if id == uuid.Nil || seen[id] {
			return fmt.Errorf("%w: invalid or repeated participant", ErrValidation)
		}
		seen[id] = true
	}
	for _, date := range []*string{input.DepartureDate, input.ArrivalDate} {
		if date != nil {
			if _, err := time.Parse("2006-01-02", *date); err != nil {
				return fmt.Errorf("%w: invalid date", ErrValidation)
			}
		}
	}
	for _, clock := range []*string{input.DepartureTime, input.ArrivalTime} {
		if clock != nil {
			if _, err := time.Parse("15:04", *clock); err != nil {
				return fmt.Errorf("%w: invalid time", ErrValidation)
			}
		}
	}
	if input.DepartureDate != nil && input.ArrivalDate != nil {
		if *input.ArrivalDate < *input.DepartureDate || (*input.ArrivalDate == *input.DepartureDate && input.DepartureTime != nil && input.ArrivalTime != nil && *input.ArrivalTime < *input.DepartureTime) {
			return fmt.Errorf("%w: arrival must not precede departure", ErrValidation)
		}
	}
	if input.Payments != nil {
		for _, payment := range *input.Payments {
			if payment.UserID == uuid.Nil || payment.AmountCents <= 0 {
				return fmt.Errorf("%w: invalid payments", ErrValidation)
			}
		}
	}
	return nil
}

func (s *Service) Create(ctx context.Context, actor, planning uuid.UUID, input Input) (Transport, error) {
	if _, err := s.member(ctx, planning, actor); err != nil {
		return Transport{}, err
	}
	if err := s.validate(ctx, planning, input); err != nil {
		return Transport{}, err
	}
	return s.repository.Create(ctx, actor, planning, input)
}

func (s *Service) Get(ctx context.Context, actor, planning, id uuid.UUID) (Transport, error) {
	if _, err := s.member(ctx, planning, actor); err != nil {
		return Transport{}, err
	}
	return s.repository.Get(ctx, planning, id)
}

func (s *Service) List(ctx context.Context, actor, planning uuid.UUID) ([]Transport, error) {
	if _, err := s.member(ctx, planning, actor); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, planning)
}

func (s *Service) editable(ctx context.Context, actor, planning, id uuid.UUID) (Transport, error) {
	member, err := s.member(ctx, planning, actor)
	if err != nil {
		return Transport{}, err
	}
	item, err := s.repository.Get(ctx, planning, id)
	if err != nil {
		return Transport{}, err
	}
	if member.Role != plannings.RoleOwner && member.Role != plannings.RoleAdmin && item.CreatedByUserID != actor {
		return Transport{}, ErrForbidden
	}
	return item, nil
}

func (s *Service) Update(ctx context.Context, actor, planning, id uuid.UUID, input Input) (Transport, error) {
	current, err := s.editable(ctx, actor, planning, id)
	if err != nil {
		return Transport{}, err
	}
	if input.Payments == nil {
		input.Payments = current.Payments
	}
	if err := s.validate(ctx, planning, input); err != nil {
		return Transport{}, err
	}
	return s.repository.Update(ctx, planning, id, input)
}

func (s *Service) Confirm(ctx context.Context, actor, planning, id uuid.UUID) (Transport, error) {
	item, err := s.editable(ctx, actor, planning, id)
	if err != nil {
		return Transport{}, err
	}
	if item.Status == "confirmed" {
		item.Status = "draft"
	} else {
		item.Status = "confirmed"
	}
	return s.repository.Update(ctx, planning, id, item.Input)
}

func (s *Service) Delete(ctx context.Context, actor, planning, id uuid.UUID) error {
	if _, err := s.editable(ctx, actor, planning, id); err != nil {
		return err
	}
	return s.repository.Delete(ctx, planning, id)
}
