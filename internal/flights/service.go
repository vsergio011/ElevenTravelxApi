package flights

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/eleventravel/eleventravel-api/internal/expenseledger"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/google/uuid"
)

type PlanningMembersLister interface {
	ListMembers(context.Context, uuid.UUID) ([]plannings.PlanningMember, error)
}
type Service struct {
	repository Repository
	membership PlanningMembership
	lookup     FlightLookupProvider
	cache      LookupCache
	now        func() time.Time
}

func NewService(repository Repository, membership PlanningMembership, lookup ...FlightLookupProvider) *Service {
	service := &Service{repository: repository, membership: membership, now: time.Now}
	if len(lookup) > 0 {
		service.lookup = lookup[0]
	}
	return service
}

func (s *Service) WithLookupCache(cache LookupCache) *Service {
	s.cache = cache
	return s
}

func (s *Service) Create(ctx context.Context, actor, planning uuid.UUID, input CreateInput) (Flight, error) {
	if _, err := s.requireMember(ctx, planning, actor); err != nil {
		return Flight{}, err
	}
	if err := s.validateUsers(ctx, planning, input.ParticipantUserIDs); err != nil {
		return Flight{}, err
	}
	if err := s.validatePaymentUsers(ctx, planning, input.Payments); err != nil {
		return Flight{}, err
	}
	if err := validateInput(input); err != nil {
		return Flight{}, err
	}
	if err := s.validatePaymentTotal(ctx, planning, input.CostDistribution, input.CostCents, input.Payments); err != nil {
		return Flight{}, err
	}
	return s.repository.Create(ctx, actor, planning, input)
}
func (s *Service) Get(ctx context.Context, actor, planning, id uuid.UUID) (Flight, error) {
	if _, err := s.requireMember(ctx, planning, actor); err != nil {
		return Flight{}, err
	}
	return s.repository.Get(ctx, planning, id)
}
func (s *Service) List(ctx context.Context, actor, planning uuid.UUID, filters Filters) ([]Flight, error) {
	if _, err := s.requireMember(ctx, planning, actor); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, planning, filters)
}
func (s *Service) Lookup(ctx context.Context, actor, planning uuid.UUID, flightNumber string, flightDate time.Time) (FlightLookup, error) {
	if _, err := s.requireMember(ctx, planning, actor); err != nil {
		return FlightLookup{}, err
	}
	if s.lookup == nil {
		return FlightLookup{}, ErrFlightLookupUnavailable
	}
	normalized := NormalizeFlightNumber(flightNumber)
	if normalized == "" {
		return FlightLookup{}, ErrFlightLookupNotFound
	}
	if s.cache != nil {
		if cached, hit, err := s.cache.Get(ctx, normalized, flightDate); err == nil && hit {
			return cached, nil
		}
	}

	result, err := s.lookup.Lookup(ctx, normalized, flightDate)
	if errors.Is(err, ErrFlightLookupNotFound) {
		nowUTC := s.now().UTC().Truncate(24 * time.Hour)
		targetUTC := time.Date(flightDate.Year(), flightDate.Month(), flightDate.Day(), 0, 0, 0, 0, time.UTC)
		if int(targetUTC.Sub(nowUTC).Hours()/24) > 14 {
			offset := (int(targetUTC.Weekday()) - int(nowUTC.Weekday()) + 7) % 7
			if offset == 0 {
				offset = 7
			}
			fallbackDate := nowUTC.AddDate(0, 0, offset)
			if fallbackResult, fbErr := s.lookup.Lookup(ctx, normalized, fallbackDate); fbErr == nil {
				fallbackResult.IsEstimated = true
				fallbackResult.MatchedDate = fallbackDate.Format("2006-01-02")
				result = fallbackResult
				err = nil
			}
		}
	}
	if err != nil {
		return FlightLookup{}, err
	}
	if s.cache != nil {
		ttl := 24 * time.Hour
		if result.IsEstimated {
			ttl = 12 * time.Hour
		}
		_ = s.cache.Set(ctx, normalized, flightDate, result, ttl)
	}
	return result, nil
}
func (s *Service) Update(ctx context.Context, actor, planning, id uuid.UUID, input UpdateInput) (Flight, error) {
	flight, err := s.repository.Get(ctx, planning, id)
	if err != nil {
		return Flight{}, err
	}
	if err = s.requireEdit(ctx, planning, actor, flight); err != nil {
		return Flight{}, err
	}
	if input.ParticipantUserIDs != nil {
		if err = s.validateUsers(ctx, planning, *input.ParticipantUserIDs); err != nil {
			return Flight{}, err
		}
	}
	if input.Payments != nil {
		if err = s.validatePaymentUsers(ctx, planning, *input.Payments); err != nil {
			return Flight{}, err
		}
	}
	if input.Segments != nil {
		copyFlight := flight
		copyFlight.Segments = *input.Segments
		if err = validateSegments(copyFlight.Segments); err != nil {
			return Flight{}, newValidationError([]response.ErrorDetail{{Field: "segments", Message: err.Error()}})
		}
	}
	merged := flight
	if input.Status != nil {
		merged.Status = *input.Status
	}
	if input.Currency != nil {
		merged.Currency = *input.Currency
	}
	if input.CostCents != nil {
		merged.CostCents = *input.CostCents
	}
	if input.CostDistribution != nil {
		merged.CostDistribution = *input.CostDistribution
	}
	if input.ParticipantUserIDs != nil {
		merged.ParticipantUserIDs = *input.ParticipantUserIDs
	}
	if input.Segments != nil {
		merged.Segments = *input.Segments
	}
	payments := flight.Payments
	if input.Payments != nil {
		payments = *input.Payments
	}
	if err = validateInput(CreateInput{Status: merged.Status, CostCents: merged.CostCents, Currency: merged.Currency, CostDistribution: merged.CostDistribution, ParticipantUserIDs: merged.ParticipantUserIDs, Payments: payments, Segments: merged.Segments}); err != nil {
		return Flight{}, err
	}
	if err = s.validatePaymentTotal(ctx, planning, merged.CostDistribution, merged.CostCents, payments); err != nil {
		return Flight{}, err
	}
	return s.repository.Update(ctx, planning, id, input)
}

func (s *Service) validatePaymentTotal(ctx context.Context, planning uuid.UUID, distribution string, amount int64, payments []Payment) error {
	lister, ok := s.membership.(PlanningMembersLister)
	if !ok {
		return ErrForbidden
	}
	members, err := lister.ListMembers(ctx, planning)
	if err != nil {
		return err
	}
	if err := expenseledger.ValidatePayments(
		expenseledger.EconomicTotal(distribution, amount, len(members)),
		payments,
	); err != nil {
		return newValidationError([]response.ErrorDetail{{
			Field:   "payments",
			Message: "payments cannot exceed the distributed total",
		}})
	}
	return nil
}
func (s *Service) Confirm(ctx context.Context, actor, planning, id uuid.UUID) (Flight, error) {
	flight, err := s.repository.Get(ctx, planning, id)
	if err != nil {
		return Flight{}, err
	}
	if err = s.requireEdit(ctx, planning, actor, flight); err != nil {
		return Flight{}, err
	}
	if flight.Status == StatusConfirmed {
		draft := StatusDraft
		return s.repository.Update(ctx, planning, id, UpdateInput{Status: &draft})
	}
	return s.repository.Confirm(ctx, planning, id)
}
func (s *Service) Delete(ctx context.Context, actor, planning, id uuid.UUID) error {
	flight, err := s.repository.Get(ctx, planning, id)
	if err != nil {
		return err
	}
	if err = s.requireEdit(ctx, planning, actor, flight); err != nil {
		return err
	}
	return s.repository.Delete(ctx, planning, id)
}

// GetConfirmedFlightBudget is consumed by bookings' existing budget service.
// It returns calculated lines, never copied expense rows, so updates are idempotent.
func (s *Service) GetConfirmedFlightBudget(ctx context.Context, planning uuid.UUID, participantIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	flights, err := s.repository.List(ctx, planning, Filters{Status: ptr(StatusConfirmed)})
	if err != nil {
		return nil, err
	}
	result := map[uuid.UUID]int64{}
	for _, id := range participantIDs {
		result[id] = 0
	}
	for _, flight := range flights {
		eligible := participantIDs
		if flight.CostDistribution == DistributionSelectedParticipants {
			eligible = flight.ParticipantUserIDs
		}
		if flight.CostDistribution == DistributionIndividual {
			for _, id := range eligible {
				result[id] += flight.CostCents
			}
			continue
		}
		if len(eligible) == 0 {
			continue
		}
		sorted := append([]uuid.UUID(nil), eligible...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })
		base := flight.CostCents / int64(len(sorted))
		remainder := flight.CostCents % int64(len(sorted))
		for i, id := range sorted {
			result[id] += base
			if int64(i) < remainder {
				result[id]++
			}
		}
	}
	return result, nil
}
func ptr(value string) *string { return &value }
func (s *Service) requireMember(ctx context.Context, planning, actor uuid.UUID) (plannings.PlanningMember, error) {
	member, err := s.membership.GetMember(ctx, planning, actor)
	if errors.Is(err, plannings.ErrMemberNotFound) {
		return plannings.PlanningMember{}, ErrForbidden
	}
	return member, err
}
func (s *Service) validateUsers(ctx context.Context, planning uuid.UUID, ids []uuid.UUID) error {
	lister, ok := s.membership.(PlanningMembersLister)
	if !ok {
		return ErrForbidden
	}
	members, err := lister.ListMembers(ctx, planning)
	if err != nil {
		return err
	}
	allowed := map[uuid.UUID]struct{}{}
	for _, member := range members {
		allowed[member.UserID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := allowed[id]; !ok {
			return ErrForbidden
		}
	}
	return nil
}

func (s *Service) validatePaymentUsers(ctx context.Context, planning uuid.UUID, payments []Payment) error {
	userIDs := make([]uuid.UUID, len(payments))
	for index, payment := range payments {
		userIDs[index] = payment.UserID
	}
	return s.validateUsers(ctx, planning, userIDs)
}

func (s *Service) requireEdit(ctx context.Context, planning, actor uuid.UUID, flight Flight) error {
	member, err := s.requireMember(ctx, planning, actor)
	if err != nil {
		return err
	}
	if member.Role == plannings.RoleOwner || member.Role == plannings.RoleAdmin || flight.CreatedByUserID == actor {
		return nil
	}
	return ErrForbidden
}
